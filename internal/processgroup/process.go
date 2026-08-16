// Package processgroup runs a subprocess in an owned operating-system process group.
package processgroup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

const terminationPollInterval = 25 * time.Millisecond

// Options configures one supervised process without exposing os/exec mutation after startup.
type Options struct {
	Dir    string
	Env    []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Process owns one subprocess and the operating-system process group created for it.
type Process struct {
	command *exec.Cmd
	done    chan struct{}

	waitMu        sync.Mutex
	waitErr       error
	descendants   descendantTracker
	terminateOnce sync.Once
	terminateDone chan struct{}
	terminateErr  error
}

// Start launches executable as the leader of a new process group.
func Start(executable string, args []string, options Options) (*Process, error) {
	if executable == "" {
		return nil, fmt.Errorf("process executable is required")
	}

	command := exec.Command(executable, args...)
	command.Dir = options.Dir
	command.Env = options.Env
	command.Stdin = options.Stdin
	command.Stdout = options.Stdout
	command.Stderr = options.Stderr
	configureProcessGroup(command)

	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", executable, err)
	}

	process := &Process{
		command:       command,
		done:          make(chan struct{}),
		terminateDone: make(chan struct{}),
	}
	process.descendants = startDescendantTracker(command.Process.Pid)
	go process.wait()
	return process, nil
}

// PID returns the process-group leader identifier.
func (process *Process) PID() int {
	if process == nil || process.command == nil || process.command.Process == nil {
		return 0
	}
	return process.command.Process.Pid
}

// Wait waits for the process-group leader or the caller's context.
func (process *Process) Wait(ctx context.Context) error {
	if process == nil {
		return fmt.Errorf("process is required")
	}
	select {
	case <-process.done:
		process.waitMu.Lock()
		defer process.waitMu.Unlock()
		return process.waitErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Terminate asks the complete process group to stop and escalates when the cleanup budget expires.
func (process *Process) Terminate(ctx context.Context) error {
	if process == nil || process.PID() == 0 {
		return nil
	}
	process.terminateOnce.Do(func() {
		go func() {
			process.terminateErr = process.terminate(ctx)
			close(process.terminateDone)
		}()
	})
	select {
	case <-process.terminateDone:
		return process.terminateErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

// terminate performs the single signal and escalation sequence shared by every cleanup caller.
func (process *Process) terminate(ctx context.Context) error {
	targets := process.descendants.Stop()
	ticker := time.NewTicker(terminationPollInterval)
	defer ticker.Stop()
	leaderDone := process.done
	leaderExited := false
	select {
	case <-leaderDone:
		leaderExited = true
		leaderDone = nil
	default:
	}

	var leaderTermErr error
	if !leaderExited {
		leaderTermErr = terminateProcessGroup(process.command)
	}
	termErr := errors.Join(leaderTermErr, terminateDescendants(targets))
	for {
		if leaderExited && descendantsTerminated(targets) {
			return ignoreMissingProcess(termErr)
		}
		select {
		case <-leaderDone:
			leaderExited = true
			leaderDone = nil
		case <-ctx.Done():
			var leaderKillErr error
			if !leaderExited {
				leaderKillErr = killProcessGroup(process.command)
			}
			killErr := errors.Join(leaderKillErr, killDescendants(targets))
			return errors.Join(ignoreMissingProcess(termErr), ignoreMissingProcess(killErr), ctx.Err())
		case <-ticker.C:
		}
	}
}

// wait records os/exec's single terminal result before notifying concurrent waiters.
func (process *Process) wait() {
	err := process.command.Wait()
	process.waitMu.Lock()
	process.waitErr = err
	process.waitMu.Unlock()
	close(process.done)
}
