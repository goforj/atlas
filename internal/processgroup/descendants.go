package processgroup

// processIdentity binds a recorded PID to its operating-system creation identity.
type processIdentity struct {
	PID       int
	GroupID   int
	StartTime uint64
}

// processTargets contains descendants observed while the supervised process was alive.
type processTargets struct {
	Processes []processIdentity
}

// descendantTracker records processes that escape the leader's original process group.
type descendantTracker interface {
	Stop() processTargets
}
