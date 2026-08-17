package codexappserver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

const fakeAppServerEnv = "ATLAS_FAKE_CODEX_APP_SERVER"

// TestClientAttributesFreshThreads verifies the adapter records effective identity and instruction provenance from Codex.
func TestClientAttributesFreshThreads(t *testing.T) {
	client := startFakeClient(t)
	defer closeClient(t, client)
	if client.UserAgent() != "fake-codex/1" {
		t.Fatalf("user agent = %q", client.UserAgent())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	first, err := client.StartThread(ctx, fakeThreadOptions(t.TempDir()))
	if err != nil {
		t.Fatalf("start first thread: %v", err)
	}
	second, err := client.StartThread(ctx, fakeThreadOptions(t.TempDir()))
	if err != nil {
		t.Fatalf("start second thread: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("fresh threads reused id %q", first.ID)
	}
	if first.Model != "gpt-test" || first.ModelProvider != "openai" {
		t.Fatalf("effective identity = %s/%s", first.ModelProvider, first.Model)
	}
	if !first.Ephemeral {
		t.Fatal("thread was not ephemeral")
	}
	if got := strings.Join(first.InstructionSources, ","); got != "/project/AGENTS.md" {
		t.Fatalf("instruction sources = %q", got)
	}
}

// TestClientStartsAndInterruptsTurn verifies request correlation across lifecycle notifications.
func TestClientStartsAndInterruptsTurn(t *testing.T) {
	client := startFakeClient(t)
	defer closeClient(t, client)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	thread, err := client.StartThread(ctx, fakeThreadOptions(t.TempDir()))
	if err != nil {
		t.Fatalf("start thread: %v", err)
	}
	turn, err := client.StartTurn(ctx, thread.ID, "add an invoice route")
	if err != nil {
		t.Fatalf("start turn: %v", err)
	}
	if turn.ID == "" {
		t.Fatal("turn id is empty")
	}
	if err := client.InterruptTurn(ctx, thread.ID, turn.ID); err != nil {
		t.Fatalf("interrupt turn: %v", err)
	}
}

// TestClientCloseIsIdempotent keeps deferred and explicit cleanup from signaling a reused process identifier.
func TestClientCloseIsIdempotent(t *testing.T) {
	client := startFakeClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Close(ctx); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := client.Close(ctx); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

// TestFakeAppServerProcess implements the versioned protocol subset without a live model.
func TestFakeAppServerProcess(t *testing.T) {
	if os.Getenv(fakeAppServerEnv) != "1" {
		return
	}
	runFakeAppServer()
	os.Exit(0)
}

// startFakeClient launches the current test binary as an isolated protocol fixture.
func startFakeClient(t *testing.T) *Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client, err := Start(ctx, StartOptions{
		Executable: os.Args[0],
		Arguments:  []string{"-test.run=TestFakeAppServerProcess"},
		Dir:        t.TempDir(),
		Env:        []string{fakeAppServerEnv + "=1"},
	})
	if err != nil {
		t.Fatalf("start fake app-server: %v", err)
	}
	return client
}

// closeClient gives cleanup an independent budget so a failed test context cannot leak the fixture.
func closeClient(t *testing.T, client *Client) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Close(ctx); err != nil {
		t.Errorf("close app-server: %v", err)
	}
}

// fakeThreadOptions keeps attribution assertions identical across fresh-thread requests.
func fakeThreadOptions(root string) ThreadOptions {
	return ThreadOptions{
		Model:          "gpt-test",
		ModelProvider:  "openai",
		Dir:            root,
		ApprovalPolicy: "never",
		Sandbox:        "read-only",
		ServiceName:    "atlas_test",
		Ephemeral:      true,
	}
}

// runFakeAppServer responds to the protocol methods required by deterministic client tests.
func runFakeAppServer() {
	scanner := bufio.NewScanner(os.Stdin)
	threadNumber := 0
	turnNumber := 0
	for scanner.Scan() {
		var request struct {
			ID     *uint64         `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(scanner.Bytes(), &request) != nil || request.ID == nil {
			continue
		}
		var result any = map[string]any{}
		switch request.Method {
		case "initialize":
			result = map[string]any{"userAgent": "fake-codex/1"}
		case "thread/start":
			threadNumber++
			result = map[string]any{
				"model":              "gpt-test",
				"modelProvider":      "openai",
				"instructionSources": []string{"/project/AGENTS.md"},
				"thread": map[string]any{
					"id":        fmt.Sprintf("thread-%d", threadNumber),
					"ephemeral": true,
				},
			}
		case "turn/start":
			turnNumber++
			result = map[string]any{"turn": map[string]any{"id": fmt.Sprintf("turn-%d", turnNumber)}}
		case "turn/interrupt":
			result = map[string]any{}
		default:
			writeFakeResponse(*request.ID, nil, map[string]any{"code": -32601, "message": "unknown method"})
			continue
		}
		writeFakeResponse(*request.ID, result, nil)
	}
}

// writeFakeResponse emits one JSONL response without test-runner formatting.
func writeFakeResponse(id uint64, result any, protocolErr any) {
	response := map[string]any{"id": id}
	if protocolErr != nil {
		response["error"] = protocolErr
	} else {
		response["result"] = result
	}
	encoded, _ := json.Marshal(response)
	_, _ = fmt.Fprintln(os.Stdout, string(encoded))
}
