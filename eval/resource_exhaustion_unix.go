//go:build aix || darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || solaris

package eval

import (
	"errors"
	"syscall"
)

// isPlatformResourceExhaustion recognizes physical and quota-backed Unix storage exhaustion.
func isPlatformResourceExhaustion(err error) bool {
	return errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT)
}
