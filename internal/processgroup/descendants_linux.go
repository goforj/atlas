//go:build linux

package processgroup

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const descendantPollInterval = 25 * time.Millisecond

// linuxDescendantTracker records immutable process identities because agent commands may create their own groups.
type linuxDescendantTracker struct {
	rootPID int
	stop    chan struct{}
	done    chan struct{}
	once    sync.Once

	mu       sync.Mutex
	observed map[int]processIdentity
}

// linuxProcess describes the process-table fields needed for ancestry and PID-reuse checks.
type linuxProcess struct {
	processIdentity
	ParentPID int
}

// startDescendantTracker begins observation before the app-server can receive an agent turn.
func startDescendantTracker(rootPID int) descendantTracker {
	tracker := &linuxDescendantTracker{
		rootPID:  rootPID,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
		observed: map[int]processIdentity{},
	}
	go tracker.run()
	return tracker
}

// Stop takes one final process-table snapshot before returning every observed identity.
func (tracker *linuxDescendantTracker) Stop() processTargets {
	tracker.once.Do(func() {
		close(tracker.stop)
		<-tracker.done
	})
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	processes := make([]processIdentity, 0, len(tracker.observed))
	for _, process := range tracker.observed {
		processes = append(processes, process)
	}
	return processTargets{Processes: processes}
}

// run samples ancestry until cleanup freezes the target set.
func (tracker *linuxDescendantTracker) run() {
	defer close(tracker.done)
	ticker := time.NewTicker(descendantPollInterval)
	defer ticker.Stop()
	tracker.capture()
	for {
		select {
		case <-ticker.C:
			tracker.capture()
		case <-tracker.stop:
			tracker.capture()
			return
		}
	}
}

// capture records every process currently descended from the app-server leader.
func (tracker *linuxDescendantTracker) capture() {
	processes := readLinuxProcessTable()
	descendants := map[int]bool{tracker.rootPID: true}
	changed := true
	for changed {
		changed = false
		for pid, process := range processes {
			if descendants[pid] || !descendants[process.ParentPID] {
				continue
			}
			descendants[pid] = true
			changed = true
		}
	}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	for pid := range descendants {
		if pid == tracker.rootPID {
			continue
		}
		if process, ok := processes[pid]; ok {
			tracker.observed[pid] = process.processIdentity
		}
	}
}

// readLinuxProcessTable returns a best-effort snapshot without making process discovery a terminal adapter error.
func readLinuxProcessTable() map[int]linuxProcess {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	processes := make(map[int]linuxProcess, len(entries))
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		process, ok := readLinuxProcess(pid)
		if ok {
			processes[pid] = process
		}
	}
	return processes
}

// readLinuxProcess parses stat after the parenthesized command so spaces and parentheses in names remain harmless.
func readLinuxProcess(pid int) (linuxProcess, bool) {
	body, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return linuxProcess{}, false
	}
	end := strings.LastIndex(string(body), ") ")
	if end < 0 {
		return linuxProcess{}, false
	}
	fields := strings.Fields(string(body[end+2:]))
	if len(fields) < 20 {
		return linuxProcess{}, false
	}
	parentPID, parentErr := strconv.Atoi(fields[1])
	groupID, groupErr := strconv.Atoi(fields[2])
	startTime, startErr := strconv.ParseUint(fields[19], 10, 64)
	if parentErr != nil || groupErr != nil || startErr != nil {
		return linuxProcess{}, false
	}
	return linuxProcess{
		processIdentity: processIdentity{PID: pid, GroupID: groupID, StartTime: startTime},
		ParentPID:       parentPID,
	}, true
}

// terminateDescendants sends graceful termination to every still-matching recorded identity and group.
func terminateDescendants(targets processTargets) error {
	return signalLinuxTargets(targets, syscall.SIGTERM)
}

// killDescendants forcefully stops every still-matching recorded identity and group.
func killDescendants(targets processTargets) error {
	return signalLinuxTargets(targets, syscall.SIGKILL)
}

// signalLinuxTargets validates creation identity before signaling so PID reuse cannot target unrelated processes.
func signalLinuxTargets(targets processTargets, signal syscall.Signal) error {
	groups := map[int]bool{}
	var result error
	for _, identity := range targets.Processes {
		current, ok := readLinuxProcess(identity.PID)
		if !ok || current.StartTime != identity.StartTime {
			continue
		}
		if current.GroupID > 0 && current.GroupID != syscall.Getpgrp() {
			groups[current.GroupID] = true
			continue
		}
		if err := syscall.Kill(identity.PID, signal); err != nil && !errors.Is(err, syscall.ESRCH) {
			result = errors.Join(result, err)
		}
	}
	for groupID := range groups {
		if err := syscall.Kill(-groupID, signal); err != nil && !errors.Is(err, syscall.ESRCH) {
			result = errors.Join(result, err)
		}
	}
	return result
}
