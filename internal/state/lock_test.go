// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/fakes3"
	"github.com/nivis-project/nivis/internal/state"
)

// openS3Locker opens an s3 store and asserts it implements Locker.
func openS3Locker(t *testing.T, srv *fakes3.Server, bucket, key string) state.Locker {
	t.Helper()
	st := openS3(t, srv, bucket, key)
	lk, ok := st.(state.Locker)
	if !ok {
		t.Fatal("s3 store should implement state.Locker")
	}
	return lk
}

// Acquire a free lock, then a second acquire is blocked with the holder's info;
// unlock releases it; a subsequent acquire succeeds.
func TestS3LockAcquireBlockRelease(t *testing.T) {
	srv := fakes3.New()
	defer srv.Close()
	bucket, key := "b", "prod/app.json"
	a := openS3Locker(t, srv, bucket, key)
	b := openS3Locker(t, srv, bucket, key)

	infoA := state.LockInfo{ID: "run-a", Who: "alice@host", Operation: "apply", Created: "2026-06-22T10:00:00Z"}
	id, err := a.Lock(infoA)
	if err != nil {
		t.Fatalf("first lock should succeed: %v", err)
	}
	if id != "run-a" {
		t.Errorf("lock id = %q, want run-a", id)
	}
	// The lock object exists in S3.
	if !srv.Has(bucket, key+".lock") {
		t.Error("lock object was not created in S3")
	}

	// A second acquire is blocked, and the error names the holder.
	_, err = b.Lock(state.LockInfo{ID: "run-b", Who: "bob@host", Operation: "apply"})
	if err == nil {
		t.Fatal("second lock should be blocked while held")
	}
	for _, want := range []string{"alice@host", "2026-06-22T10:00:00Z", "force-unlock"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("blocked-lock error %q should mention %q", err.Error(), want)
		}
	}

	// Release with the held id, then a re-acquire succeeds.
	if err := a.Unlock(id); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if srv.Has(bucket, key+".lock") {
		t.Error("lock object should be gone after unlock")
	}
	if _, err := b.Lock(state.LockInfo{ID: "run-b", Who: "bob@host", Operation: "apply"}); err != nil {
		t.Fatalf("re-acquire after release should succeed: %v", err)
	}
}

// Unlock with a mismatched id is refused (don't drop someone else's lock).
func TestS3UnlockWrongIDRefused(t *testing.T) {
	srv := fakes3.New()
	defer srv.Close()
	bucket, key := "b", "k"
	lk := openS3Locker(t, srv, bucket, key)

	if _, err := lk.Lock(state.LockInfo{ID: "real", Who: "alice@host", Operation: "apply"}); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if err := lk.Unlock("not-the-real-id"); err == nil {
		t.Fatal("unlock with a wrong id should be refused")
	}
	if !srv.Has(bucket, key+".lock") {
		t.Error("a refused unlock must not delete the lock")
	}
}

// ForceUnlock clears a held lock unconditionally; unlock of an absent lock is a
// no-op.
func TestS3ForceUnlock(t *testing.T) {
	srv := fakes3.New()
	defer srv.Close()
	bucket, key := "b", "k"
	lk := openS3Locker(t, srv, bucket, key)

	if _, err := lk.Lock(state.LockInfo{ID: "x", Who: "alice@host", Operation: "apply"}); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if err := lk.ForceUnlock(); err != nil {
		t.Fatalf("force-unlock: %v", err)
	}
	if srv.Has(bucket, key+".lock") {
		t.Error("force-unlock should remove the lock")
	}
	// Unlocking an absent lock is a no-op (not an error).
	if err := lk.Unlock("anything"); err != nil {
		t.Errorf("unlock of an absent lock should be a no-op, got %v", err)
	}
}

// NewLockInfo fills the holder fields and a unique id.
func TestNewLockInfo(t *testing.T) {
	a := state.NewLockInfo("apply")
	if a.ID == "" || a.Who == "" || a.Operation != "apply" || a.Created == "" {
		t.Errorf("NewLockInfo incomplete: %+v", a)
	}
}
