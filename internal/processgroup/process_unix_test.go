//go:build linux

package processgroup

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestTerminateStopsLeaderAndDescendant proves cleanup targets the owned group rather than only the direct child.
func TestTerminateStopsLeaderAndDescendant(t *testing.T) {
	root := t.TempDir()
	childPIDPath := filepath.Join(root, "child.pid")
	command := "setsid sleep 300 & child=$!; printf '%s' \"$child\" > \"$1\"; wait"
	process, err := Start("/bin/sh", []string{"-c", command, "atlas-process-group", childPIDPath}, Options{
		Dir:    root,
		Env:    []string{"PATH=/usr/bin:/bin"},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	if err != nil {
		t.Fatalf("start process group: %v", err)
	}

	childPID := waitForChildPID(t, childPIDPath)
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := process.Terminate(cleanupCtx); err != nil {
		t.Fatalf("terminate process group: %v", err)
	}
	if !waitForProcessExit(process.PID(), time.Second) {
		t.Fatalf("leader %d survived cleanup", process.PID())
	}
	if !waitForProcessExit(childPID, time.Second) {
		t.Fatalf("descendant %d survived cleanup", childPID)
	}
}

// TestTerminateEscalatesAfterLeaderExit proves tracked, separately grouped descendants cannot outlive a SIGTERM-resistant leader exit.
func TestTerminateEscalatesAfterLeaderExit(t *testing.T) {
	root := t.TempDir()
	childPIDPath := filepath.Join(root, "child.pid")
	command := "setsid \"$1\" -c 'trap \"\" TERM; printf \"%s\" \"$$\" > \"$1\"; exec sleep 300' atlas-child \"$2\" & while [ ! -s \"$2\" ]; do sleep 0.01; done; sleep 0.15; exit"
	process, err := Start("/bin/sh", []string{"-c", command, "atlas-process-group", "/bin/sh", childPIDPath}, Options{
		Dir: root,
		Env: []string{"PATH=/usr/bin:/bin"},
	})
	if err != nil {
		t.Fatalf("start process group: %v", err)
	}

	childPID := waitForChildPID(t, childPIDPath)
	waitCtx, cancelWait := context.WithTimeout(context.Background(), time.Second)
	defer cancelWait()
	if err := process.Wait(waitCtx); err != nil {
		t.Fatalf("wait for leader exit: %v", err)
	}
	if !processAlive(childPID) {
		t.Fatalf("SIGTERM-resistant descendant %d exited before cleanup", childPID)
	}

	cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancelCleanup()
	started := time.Now()
	err = process.Terminate(cleanupCtx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Terminate() error = %v, want cleanup deadline", err)
	}
	if elapsed := time.Since(started); elapsed < 100*time.Millisecond {
		t.Fatalf("Terminate() returned after %s before escalation", elapsed)
	}
	if !waitForProcessExit(childPID, time.Second) {
		t.Fatalf("SIGTERM-resistant descendant %d survived escalation", childPID)
	}
}

// TestWaitReturnsProcessFailure preserves the leader's terminal status for adapter diagnostics.
func TestWaitReturnsProcessFailure(t *testing.T) {
	process, err := Start("/bin/sh", []string{"-c", "exit 17"}, Options{})
	if err != nil {
		t.Fatalf("start failing process: %v", err)
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = process.Wait(waitCtx)
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 17 {
		t.Fatalf("Wait() error = %v, want exit code 17", err)
	}
}

// TestTerminateReturnsAfterEscalationWithoutWaitingForCompletion ensures cleanup callers cannot leak behind an unreported process exit.
func TestTerminateReturnsAfterEscalationWithoutWaitingForCompletion(t *testing.T) {
	command := exec.Command("/bin/sh", "-c", "sleep 300")
	configureProcessGroup(command)
	if err := command.Start(); err != nil {
		t.Fatalf("start process: %v", err)
	}
	defer command.Wait()

	process := &Process{
		command:       command,
		done:          make(chan struct{}),
		descendants:   emptyDescendantTracker{},
		terminateDone: make(chan struct{}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := make(chan error, 1)
	go func() {
		result <- process.Terminate(ctx)
	}()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Terminate() error = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Terminate() waited for a process completion signal after kill escalation")
	}
}

// TestTerminateSkipsReapedLeader prevents os/exec's completed identity from becoming a reused PID signal target.
func TestTerminateSkipsReapedLeader(t *testing.T) {
	command := exec.Command("/bin/sh", "-c", "exit 0")
	configureProcessGroup(command)
	if err := command.Start(); err != nil {
		t.Fatalf("start process: %v", err)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("wait for process: %v", err)
	}
	if err := terminateProcessGroup(command); !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("terminate reaped process = %v, want os.ErrProcessDone", err)
	}
	if err := killProcessGroup(command); !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("kill reaped process = %v, want os.ErrProcessDone", err)
	}
}

// TestSignalLinuxTargetsRejectsStaleIdentity proves a pidfd is not enough unless the observed creation identity still matches.
func TestSignalLinuxTargetsRejectsStaleIdentity(t *testing.T) {
	command := exec.Command("/bin/sh", "-c", "exec sleep 300")
	if err := command.Start(); err != nil {
		t.Fatalf("start process: %v", err)
	}
	defer func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	}()
	process, ok := readLinuxProcess(command.Process.Pid)
	if !ok {
		t.Fatalf("read process identity for %d", command.Process.Pid)
	}
	stale := process.processIdentity
	stale.StartTime++
	if err := signalLinuxTargets(processTargets{Processes: []processIdentity{stale}}, syscall.SIGTERM); err != nil {
		t.Fatalf("signal stale identity: %v", err)
	}
	if !processAlive(command.Process.Pid) {
		t.Fatal("stale identity signaled the live process")
	}
}

// emptyDescendantTracker isolates the timeout regression from Linux process-table polling.
type emptyDescendantTracker struct{}

// Stop returns no separately grouped descendants for the controlled direct child used by this regression test.
func (emptyDescendantTracker) Stop() processTargets {
	return processTargets{}
}

// waitForChildPID waits for the helper shell to publish the descendant before cancellation begins.
func waitForChildPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		body, err := os.ReadFile(path)
		if err == nil && strings.TrimSpace(string(body)) != "" {
			pid, parseErr := strconv.Atoi(strings.TrimSpace(string(body)))
			if parseErr != nil {
				t.Fatalf("parse child pid: %v", parseErr)
			}
			return pid
		}
		if err != nil && !os.IsNotExist(err) {
			t.Fatalf("read child pid: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for child pid")
	return 0
}

// processAlive reports whether a Unix process still accepts signal zero.
func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// waitForProcessExit tolerates the short interval between group signaling and orphan reaping.
func waitForProcessExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return !processAlive(pid)
}
