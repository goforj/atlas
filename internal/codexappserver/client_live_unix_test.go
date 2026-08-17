//go:build !windows

package codexappserver

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
)

// TestLiveCodexAppServerFeasibility proves freshness, attribution, interruption, and descendant cleanup against an installed Codex CLI.
func TestLiveCodexAppServerFeasibility(t *testing.T) {
	if os.Getenv("ATLAS_LIVE_CODEX") != "1" {
		t.Skip("set ATLAS_LIVE_CODEX=1 to run the live Codex feasibility test")
	}
	executable, err := exec.LookPath("codex")
	if err != nil {
		t.Fatalf("resolve Codex executable: %v", err)
	}
	root := t.TempDir()
	if _, err := git.PlainInit(root, false); err != nil {
		t.Fatalf("initialize disposable repository: %v", err)
	}
	codexState := prepareLiveCodexState(t)

	startCtx, startCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer startCancel()
	client, err := Start(startCtx, StartOptions{
		Executable: executable,
		Dir:        root,
		Env:        environmentWithCodexState(os.Environ(), codexState),
	})
	if err != nil {
		t.Fatalf("start live Codex app-server: %v", err)
	}
	closed := false
	defer func() {
		if closed {
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if err := client.Close(cleanupCtx); err != nil {
			t.Errorf("close live Codex app-server: %v", err)
		}
	}()
	events := &liveEventLog{}
	go events.capture(client.Notifications())

	model := strings.TrimSpace(os.Getenv("ATLAS_LIVE_CODEX_MODEL"))
	if model == "" {
		model = "gpt-5.6-sol"
	}
	threadOptions := ThreadOptions{
		Model:          model,
		ModelProvider:  "openai",
		Dir:            root,
		ApprovalPolicy: "never",
		Sandbox:        "danger-full-access",
		ServiceName:    "goforj_atlas_eval_feasibility",
		Ephemeral:      true,
	}
	first := startAttributedLiveThread(t, client, threadOptions)
	second := startAttributedLiveThread(t, client, threadOptions)
	if first.ID == second.ID {
		t.Fatalf("fresh live threads reused id %q", first.ID)
	}

	leaderPath := filepath.Join(root, "command-leader.pid")
	childPath := filepath.Join(root, "command-child.pid")
	prompt := fmt.Sprintf(
		"Run exactly this command and wait for it to finish. Do not run anything else: sh -c 'echo $$ > %s; sleep 300 & echo $! > %s; wait'",
		strconv.Quote(leaderPath),
		strconv.Quote(childPath),
	)
	turnCtx, turnCancel := context.WithTimeout(context.Background(), 30*time.Second)
	turn, err := client.StartTurn(turnCtx, second.ID, prompt)
	turnCancel()
	if err != nil {
		t.Fatalf("start live Codex turn: %v", err)
	}
	leaderPID, err := waitForLivePID(leaderPath, 60*time.Second)
	if err != nil {
		t.Fatalf("%v\nCodex notifications:\n%s\nCodex stderr:\n%s", err, events.String(), client.Stderr())
	}
	childPID, err := waitForLivePID(childPath, 5*time.Second)
	if err != nil {
		t.Fatalf("%v\nCodex notifications:\n%s\nCodex stderr:\n%s", err, events.String(), client.Stderr())
	}

	interruptCtx, interruptCancel := context.WithTimeout(context.Background(), 10*time.Second)
	interruptErr := client.InterruptTurn(interruptCtx, second.ID, turn.ID)
	interruptCancel()
	if interruptErr != nil {
		t.Fatalf("interrupt live Codex turn: %v", interruptErr)
	}
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := client.Close(cleanupCtx); err != nil {
		cleanupCancel()
		t.Fatalf("close live Codex app-server: %v", err)
	}
	cleanupCancel()
	closed = true
	if !waitForLiveProcessExit(leaderPID, 5*time.Second) {
		t.Fatalf("Codex command leader %d survived cancellation", leaderPID)
	}
	if !waitForLiveProcessExit(childPID, 5*time.Second) {
		t.Fatalf("Codex command descendant %d survived cancellation", childPID)
	}
}

// prepareLiveCodexState copies only the login material required by this opt-in diagnostic run.
func prepareLiveCodexState(t *testing.T) string {
	t.Helper()
	sourceRoot := strings.TrimSpace(os.Getenv("CODEX_HOME"))
	if sourceRoot == "" {
		userRoot, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("resolve Codex login root: %v", err)
		}
		sourceRoot = filepath.Join(userRoot, ".codex")
	}
	auth, err := os.ReadFile(filepath.Join(sourceRoot, "auth.json"))
	if err != nil {
		t.Fatalf("read Codex login for isolated feasibility run: %v", err)
	}
	targetRoot := filepath.Join(t.TempDir(), "codex-state")
	if err := os.MkdirAll(targetRoot, 0o700); err != nil {
		t.Fatalf("create isolated Codex state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetRoot, "auth.json"), auth, 0o600); err != nil {
		t.Fatalf("write isolated Codex login: %v", err)
	}
	return targetRoot
}

// environmentWithCodexState prevents the app-server from discovering maintainer-global instructions and configuration.
func environmentWithCodexState(parent []string, stateRoot string) []string {
	environment := make([]string, 0, len(parent)+1)
	for _, value := range parent {
		if strings.HasPrefix(value, "CODEX_HOME=") {
			continue
		}
		environment = append(environment, value)
	}
	return append(environment, "CODEX_HOME="+stateRoot)
}

// startAttributedLiveThread rejects silent changes to the resolved model or guidance sources.
func startAttributedLiveThread(t *testing.T, client *Client, options ThreadOptions) Thread {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	thread, err := client.StartThread(ctx, options)
	if err != nil {
		t.Fatalf("start live Codex thread: %v\n%s", err, client.Stderr())
	}
	if thread.Model != options.Model || thread.ModelProvider != options.ModelProvider {
		t.Fatalf("effective model = %s/%s, want %s/%s", thread.ModelProvider, thread.Model, options.ModelProvider, options.Model)
	}
	if !thread.Ephemeral {
		t.Fatal("live Codex thread was persisted")
	}
	if len(thread.InstructionSources) != 0 {
		t.Fatalf("unexpected instruction sources in isolated repository: %v", thread.InstructionSources)
	}
	return thread
}

// waitForLivePID waits for the agent-issued command to expose its supervised process identity.
func waitForLivePID(path string, timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		body, err := os.ReadFile(path)
		if err == nil {
			pid, parseErr := strconv.Atoi(strings.TrimSpace(string(body)))
			if parseErr != nil {
				return 0, fmt.Errorf("parse live process pid: %w", parseErr)
			}
			return pid, nil
		}
		if !os.IsNotExist(err) {
			return 0, fmt.Errorf("read live process pid: %w", err)
		}
		time.Sleep(25 * time.Millisecond)
	}
	return 0, fmt.Errorf("timed out waiting for live process pid at %s", path)
}

// waitForLiveProcessExit verifies no process remains reachable after turn and app-server cleanup.
func waitForLiveProcessExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) != nil {
			return true
		}
		time.Sleep(25 * time.Millisecond)
	}
	return syscall.Kill(pid, 0) != nil
}

// liveEventLog retains a bounded protocol tail so a failed opt-in spike explains itself.
type liveEventLog struct {
	mu     sync.Mutex
	events []string
}

// capture drains notifications continuously so protocol backpressure cannot stall the agent.
func (log *liveEventLog) capture(notifications <-chan Notification) {
	for notification := range notifications {
		line := notification.Method
		if len(notification.Params) > 0 {
			line += " " + string(notification.Params)
		}
		log.mu.Lock()
		log.events = append(log.events, line)
		if len(log.events) > 80 {
			log.events = append([]string(nil), log.events[len(log.events)-80:]...)
		}
		log.mu.Unlock()
	}
}

// String returns the retained event tail without exposing mutable capture state.
func (log *liveEventLog) String() string {
	log.mu.Lock()
	defer log.mu.Unlock()
	return strings.Join(log.events, "\n")
}
