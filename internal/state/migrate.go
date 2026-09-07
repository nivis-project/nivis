// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"bytes"
	"errors"
	"fmt"
)

// Migration of a whole state document from one backend to another. It is expressed
// entirely through the document-level seam (Snapshot/Restore) plus the optional
// Locker and Remover seams, so it works for ANY pair of backends without
// per-backend migration code.

// MigrateOptions configures a migration.
type MigrateOptions struct {
	// Force overrides the destination guard: it allows overwriting a destination
	// that holds different resources, or content that is not a state document.
	Force bool
	// Operation labels the lock (who holds it and why). Defaults to "state migrate".
	Operation string
}

// MigrateResult reports what a successful migration did.
type MigrateResult struct {
	// Resources is how many resource states the migrated document holds.
	Resources int
	// Resumed is true when the destination already held exactly this document, so
	// the copy had landed in an earlier, interrupted run and only the source
	// removal remained.
	Resumed bool
}

// ConflictError reports that the destination already holds state the migration
// would destroy. It carries both sides' resource counts so a caller can report
// them, and is returned BEFORE anything is written.
type ConflictError struct {
	// SourceResources and DestinationResources are the resource counts on each side.
	SourceResources      int
	DestinationResources int
	// Unparseable is true when the destination holds content that is not a Nivis
	// state document at all (so the counts are not meaningful).
	Unparseable bool
}

func (e *ConflictError) Error() string {
	if e.Unparseable {
		return "the destination holds content that is not a Nivis state document; " +
			"refusing to overwrite it (pass --force to overwrite it anyway)"
	}
	return fmt.Sprintf("the destination already holds %d resource(s) and the source holds %d; "+
		"refusing to overwrite state that is not a copy of the source (pass --force to overwrite it anyway)",
		e.DestinationResources, e.SourceResources)
}

// destinationState is the destination's content as the guard sees it: enough to
// decide, and nothing more, so the decision itself stays a pure function.
type destinationState struct {
	// Parseable is false when the destination holds content that is not a state
	// document (in which case Snapshot and Resources are meaningless).
	Parseable bool
	// Snapshot is the destination's canonical document bytes.
	Snapshot []byte
	// Resources is how many resource states the destination document holds.
	Resources int
}

// migrateVerdict is the guard's decision.
type migrateVerdict int

const (
	// verdictWrite: copy the source over the destination.
	verdictWrite migrateVerdict = iota
	// verdictResume: the destination is already this exact document (an
	// interrupted migration), so skip the copy and finish the removal.
	verdictResume
)

// evaluateGuard decides whether a migration may proceed, given the source document
// and what the destination currently holds. It is pure: no I/O, so every row of
// the guard table is directly testable.
//
// The guard keys on CONTENT, not on whether a destination object exists:
//
//   - absent or empty document      -> write (the fresh-backend case; refusing here
//     would make the common bootstrap need --force,
//     which would train everyone to always pass it)
//   - identical to the source       -> resume (the copy landed, the removal did not)
//   - different resources           -> refuse unless forced
//   - not a state document at all   -> refuse unless forced (unrecognized content
//     may not be ours to destroy)
func evaluateGuard(srcSnapshot []byte, dst destinationState, force bool) (migrateVerdict, error) {
	srcCount, err := countDocumentResources(srcSnapshot)
	if err != nil {
		return verdictWrite, err
	}
	if !dst.Parseable {
		if force {
			return verdictWrite, nil
		}
		return verdictWrite, &ConflictError{SourceResources: srcCount, Unparseable: true}
	}
	if dst.Resources == 0 {
		return verdictWrite, nil
	}
	if bytes.Equal(srcSnapshot, dst.Snapshot) {
		return verdictResume, nil
	}
	if force {
		return verdictWrite, nil
	}
	return verdictWrite, &ConflictError{SourceResources: srcCount, DestinationResources: dst.Resources}
}

// countDocumentResources reports how many resource states a canonical document
// holds.
func countDocumentResources(data []byte) (int, error) {
	doc, err := parseDocument(data)
	if err != nil {
		return 0, err
	}
	return len(doc.Resources), nil
}

// inspectDestination reads the destination through the document seam and
// classifies it for the guard. Content that is not a state document is reported as
// unparseable (a guard input), while a genuine backend failure (unreachable
// location, denied credentials) is returned as an error — the two must not be
// conflated.
func inspectDestination(dst Store) (destinationState, error) {
	snap, err := dst.Snapshot()
	if err != nil {
		if errors.Is(err, ErrInvalidDocument) {
			return destinationState{Parseable: false}, nil
		}
		return destinationState{}, err
	}
	count, err := countDocumentResources(snap)
	if err != nil {
		// Snapshot returned bytes that do not parse: still unrecognized content.
		return destinationState{Parseable: false}, nil
	}
	return destinationState{Parseable: true, Snapshot: snap, Resources: count}, nil
}

// Migrate moves the whole state document from src to dst and removes it from src.
//
// The order is the safety argument, not an implementation detail:
//
//  1. lock the destination, then the source (each side that supports locking),
//     in that fixed order so two opposing migrations fail instead of deadlocking
//  2. snapshot the source
//  3. evaluate the destination guard; abort before any write if it refuses
//  4. restore the snapshot into the destination
//  5. re-read the destination and VERIFY it matches the snapshot
//  6. remove the source ONLY after that verification passes
//  7. release both locks, on every path including failure
//
// A failure anywhere before step 6 leaves the source document complete, so the
// migration is safe to re-run: the destination then holds an identical document
// and the guard resumes rather than reporting a conflict.
func Migrate(src, dst Store, opts MigrateOptions) (res MigrateResult, err error) {
	operation := opts.Operation
	if operation == "" {
		operation = "state migrate"
	}

	// 1. Both locks, destination first.
	release, err := lockPair(dst, src, operation)
	if err != nil {
		return MigrateResult{}, preRemoval(err)
	}
	defer func() {
		if rerr := release(); rerr != nil && err == nil {
			err = rerr
		}
	}()

	// The source must be removable before anything is written: discovering this
	// after the copy would leave two live copies of the state of record.
	remover, ok := src.(Remover)
	if !ok {
		return MigrateResult{}, preRemoval(fmt.Errorf(
			"state: migrate: the source backend does not support removing its state document"))
	}

	// 2. Snapshot the source.
	srcSnapshot, err := src.Snapshot()
	if err != nil {
		return MigrateResult{}, preRemoval(fmt.Errorf("state: migrate: read the source state: %w", err))
	}
	count, err := countDocumentResources(srcSnapshot)
	if err != nil {
		return MigrateResult{}, preRemoval(fmt.Errorf("state: migrate: the source state is not a valid document: %w", err))
	}

	// 3. Guard the destination.
	dstState, err := inspectDestination(dst)
	if err != nil {
		return MigrateResult{}, preRemoval(fmt.Errorf("state: migrate: read the destination state: %w", err))
	}
	verdict, err := evaluateGuard(srcSnapshot, dstState, opts.Force)
	if err != nil {
		return MigrateResult{}, preRemoval(err)
	}

	// 4. Write, unless the destination already holds exactly this document.
	if verdict != verdictResume {
		if err := dst.Restore(srcSnapshot); err != nil {
			return MigrateResult{}, preRemoval(fmt.Errorf("state: migrate: write the destination state: %w", err))
		}
	}

	// 5. Verify the destination really holds what we sent, before destroying the
	// only other copy.
	written, err := dst.Snapshot()
	if err != nil {
		return MigrateResult{}, preRemoval(fmt.Errorf("state: migrate: verify the destination state: %w", err))
	}
	if !bytes.Equal(written, srcSnapshot) {
		return MigrateResult{}, preRemoval(fmt.Errorf(
			"state: migrate: the destination does not match the source after writing it, so the migration was abandoned"))
	}

	// 6. Only now is the source redundant.
	if err := remover.Remove(); err != nil {
		return MigrateResult{}, fmt.Errorf("state: migrate: the state was copied and verified, but removing the source failed: %w\n"+
			"  The destination now holds the state; remove the source by hand, or re-run the migration (it will resume)", err)
	}

	return MigrateResult{Resources: count, Resumed: verdict == verdictResume}, nil
}

// preRemoval annotates a failure that happened before the source was removed, so
// every such error tells the user their state is intact and the command is safe to
// re-run.
func preRemoval(err error) error {
	return fmt.Errorf("%w\n  The source state is unchanged, so nothing was lost and the migration can be re-run", err)
}

// lockPair acquires the advisory lock on each side that supports locking,
// destination first and source second — a FIXED order, so two migrations running
// in opposite directions fail with the holder error instead of deadlocking. It
// returns a release function that unlocks in reverse order; on a partial failure
// it releases what it took before returning the error.
func lockPair(dst, src Store, operation string) (func() error, error) {
	var releases []func() error
	release := func() error {
		var firstErr error
		for i := len(releases) - 1; i >= 0; i-- {
			if err := releases[i](); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		releases = nil
		return firstErr
	}

	for _, side := range []struct {
		store Store
		what  string
	}{{dst, "destination"}, {src, "source"}} {
		lk, ok := side.store.(Locker)
		if !ok {
			continue // an unlockable backend (the local file store) is simply not locked
		}
		id, err := lk.Lock(NewLockInfo(operation))
		if err != nil {
			_ = release()
			return func() error { return nil }, fmt.Errorf("state: migrate: lock the %s state: %w", side.what, err)
		}
		releases = append(releases, func() error { return lk.Unlock(id) })
	}
	return release, nil
}
