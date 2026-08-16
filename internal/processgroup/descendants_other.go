//go:build !linux

package processgroup

// noopDescendantTracker leaves complete job ownership ineligible on platforms without a qualified tracker.
type noopDescendantTracker struct{}

// startDescendantTracker returns a no-op tracker so diagnostic leader cleanup remains portable.
func startDescendantTracker(int) descendantTracker {
	return noopDescendantTracker{}
}

// Stop returns no additional targets on platforms without descendant observation.
func (noopDescendantTracker) Stop() processTargets {
	return processTargets{}
}

// descendantsTerminated reports success because unsupported platforms have no tracked descendants.
func descendantsTerminated(processTargets) bool {
	return true
}

// terminateDescendants has no targets when descendant observation is unavailable.
func terminateDescendants(processTargets) error {
	return nil
}

// killDescendants has no targets when descendant observation is unavailable.
func killDescendants(processTargets) error {
	return nil
}
