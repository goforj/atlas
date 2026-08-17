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

// terminateProcessGroup requests graceful shutdown from the leader and every descendant that retained the group.
func terminateProcessGroup(command *exec.Cmd) error {
	return syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
}

// killProcessGroup stops descendants that did not honor graceful termination.
func killProcessGroup(command *exec.Cmd) error {
	return syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
}

// ignoreMissingProcess treats an already-empty process group as successful cleanup.
func ignoreMissingProcess(err error) error {
	if err == nil || errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
