//go:build !aix && !darwin && !dragonfly && !freebsd && !illumos && !linux && !netbsd && !openbsd && !solaris && !windows

package eval

// isPlatformResourceExhaustion has no platform errno mapping outside the supported Unix and Windows targets.
func isPlatformResourceExhaustion(error) bool {
	return false
}
