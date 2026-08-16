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
