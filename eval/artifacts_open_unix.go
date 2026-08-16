//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package eval

import (
	"os"
	"syscall"
)

// openArtifactFileDescriptor prevents a special-file swap from blocking before its descriptor can be validated.
func openArtifactFileDescriptor(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
