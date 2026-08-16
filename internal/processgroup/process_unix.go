//go:build !windows

package processgroup

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// configureProcessGroup prevents supervisor cancellation from signaling its own process group.
func configureProcessGroup(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminateProcessGroup uses os.Process identity state so a reaped leader PID cannot signal an unrelated reused process.
func terminateProcessGroup(command *exec.Cmd) error {
	return command.Process.Signal(syscall.SIGTERM)
}

// killProcessGroup preserves the same identity-safe leader boundary during escalation.
func killProcessGroup(command *exec.Cmd) error {
	return command.Process.Kill()
}

// ignoreMissingProcess treats an already-empty process group as successful cleanup.
func ignoreMissingProcess(err error) error {
	if err == nil || errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
