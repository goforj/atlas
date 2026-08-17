//go:build windows

package processgroup

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// configureProcessGroup gives the child a distinct console process group for diagnostic cleanup.
func configureProcessGroup(command *exec.Cmd) {
	const createNewProcessGroup = 0x00000200
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup}
}

// terminateProcessGroup falls back to terminating the leader because authoritative Windows job ownership is not yet supported.
func terminateProcessGroup(command *exec.Cmd) error {
	return command.Process.Kill()
}

// killProcessGroup repeats leader termination when graceful diagnostic cleanup did not complete.
func killProcessGroup(command *exec.Cmd) error {
	return command.Process.Kill()
}

// ignoreMissingProcess treats an already-exited process as successful cleanup.
func ignoreMissingProcess(err error) error {
	if err == nil || errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}
