//go:build aix || darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || solaris

package eval

import (
	"errors"
	"fmt"
	"syscall"
	"testing"
)

// TestIsResourceExhaustionRecognizesWrappedENOSPC keeps disk-full classification independent from error decoration.
func TestIsResourceExhaustionRecognizesWrappedENOSPC(t *testing.T) {
	for _, cause := range []error{syscall.ENOSPC, syscall.EDQUOT} {
		if !IsResourceExhaustion(fmt.Errorf("persist artifacts: %w", cause)) {
			t.Fatalf("IsResourceExhaustion() did not recognize %v", cause)
		}
	}
	if IsResourceExhaustion(errors.New("disk full")) {
		t.Fatal("IsResourceExhaustion() classified an untyped error")
	}
}
