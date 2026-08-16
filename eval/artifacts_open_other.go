//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package eval

import "os"

// openArtifactFileDescriptor opens through the anchored root on platforms without Unix FIFO semantics.
func openArtifactFileDescriptor(root *os.Root, name string) (*os.File, error) {
	return root.Open(name)
}
