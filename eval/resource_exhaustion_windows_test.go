//go:build windows

package eval

import (
	"errors"
	"fmt"
	"syscall"
	"testing"
)

// TestIsResourceExhaustionRecognizesWindowsDiskFullErrors accepts both Windows disk-full error forms without matching unrelated errno values.
func TestIsResourceExhaustionRecognizesWindowsDiskFullErrors(t *testing.T) {
	for _, errno := range []syscall.Errno{windowsErrorHandleDiskFull, windowsErrorDiskFull} {
		if !IsResourceExhaustion(fmt.Errorf("persist artifacts: %w", errno)) {
			t.Fatalf("IsResourceExhaustion() did not recognize %d", errno)
		}
	}
	if IsResourceExhaustion(errors.New("disk full")) {
		t.Fatal("IsResourceExhaustion() classified an untyped error")
	}
}
