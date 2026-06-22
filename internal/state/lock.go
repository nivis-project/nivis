// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"fmt"
	"os"
	"os/user"
	"time"
)

// Locker is the OPTIONAL advisory-lock seam a Store backend MAY implement so two
// concurrent mutating runs cannot corrupt shared state. The Store interface itself
// is unchanged; a backend that does not implement Locker is simply unlocked. The
// CLI acquires the lock around apply/destroy and releases it after the run.
type Locker interface {
	// Lock acquires the advisory lock, recording info about the holder. It returns
	// the held lock's id (used to release exactly this lock). It fails with an
	// actionable error naming the current holder if the lock is already held.
	Lock(info LockInfo) (lockID string, err error)
	// Unlock releases a lock previously acquired with the given id. It SHALL refuse
	// to release a lock whose id does not match (so a stale release cannot drop
	// another run's lock).
	Unlock(lockID string) error
	// ForceUnlock removes the lock unconditionally (the escape hatch for a stuck
	// lock left by a crashed run). It is a no-op if no lock is held.
	ForceUnlock() error
}

// LockInfo records who holds a state lock and why, so a blocked run can report it.
type LockInfo struct {
	ID        string `json:"id"`        // unique per acquisition
	Who       string `json:"who"`       // user@host
	PID       int    `json:"pid"`       // process id
	Operation string `json:"operation"` // e.g. "apply", "destroy"
	Created   string `json:"created"`   // RFC3339 timestamp
}

// NewLockInfo builds a LockInfo for the current process and the given operation.
// The id combines host/pid/time so it is unique per acquisition without needing a
// random source.
func NewLockInfo(operation string) LockInfo {
	host, _ := os.Hostname()
	if host == "" {
		host = "unknown-host"
	}
	who := host
	if u, err := user.Current(); err == nil && u.Username != "" {
		who = u.Username + "@" + host
	}
	pid := os.Getpid()
	now := time.Now().UTC()
	return LockInfo{
		ID:        fmt.Sprintf("%s-%d-%d", host, pid, now.UnixNano()),
		Who:       who,
		PID:       pid,
		Operation: operation,
		Created:   now.Format(time.RFC3339),
	}
}

// describe renders a held lock for an error message: "<who> since <created> for
// <operation>". Used when a second run is blocked.
func (li LockInfo) describe() string {
	op := li.Operation
	if op == "" {
		op = "an operation"
	}
	who := li.Who
	if who == "" {
		who = "another run"
	}
	since := li.Created
	if since == "" {
		since = "an unknown time"
	}
	return fmt.Sprintf("%s since %s for %q", who, since, op)
}
