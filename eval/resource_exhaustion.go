package eval

// IsResourceExhaustion reports whether an operation failed because its shared storage resource is full.
func IsResourceExhaustion(err error) bool {
	return isPlatformResourceExhaustion(err)
}
