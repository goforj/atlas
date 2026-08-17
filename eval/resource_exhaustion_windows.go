//go:build windows

package eval

import (
	"errors"
	"syscall"
)

const (
	windowsErrorHandleDiskFull syscall.Errno = 39
	windowsErrorDiskFull       syscall.Errno = 112
)

// isPlatformResourceExhaustion recognizes the Windows errors returned for a full disk or full file handle.
func isPlatformResourceExhaustion(err error) bool {
	return errors.Is(err, windowsErrorHandleDiskFull) || errors.Is(err, windowsErrorDiskFull)
}
