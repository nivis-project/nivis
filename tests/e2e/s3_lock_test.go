// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package e2e_test

import (
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/fakes3"
	"github.com/nivis-project/nivis/internal/state"
)

// TestS3StateLockMutualExclusion proves B2 end to end against the hermetic fake
// S3: one run holds the state lock, a second run's acquire is rejected with the
// holder's info, and after release the lock can be re-acquired. This is the
// concurrency guarantee that lets a team share the S3 backend safely.
func TestS3StateLockMutualExclusion(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")

	srv := fakes3.New()
	defer srv.Close()
	backend := map[string]interface{}{
		"type": "s3", "bucket": "team-state", "key": "prod/app.json", "region": "us-east-1", "endpoint": srv.URL(),
	}
	openLocker := func() state.Locker {
		st, err := state.OpenBackend(backend, "")
		if err != nil {
			t.Fatalf("open s3 backend: %v", err)
		}
		lk, ok := st.(state.Locker)
		if !ok {
			t.Fatal("s3 backend should implement state.Locker")
		}
		return lk
	}

	runA := openLocker()
	runB := openLocker()

	// Run A acquires the lock.
	infoA := state.NewLockInfo("apply")
	idA, err := runA.Lock(infoA)
	if err != nil {
		t.Fatalf("run A acquire: %v", err)
	}

	// Run B is rejected while A holds it, with A's holder info surfaced.
	_, err = runB.Lock(state.NewLockInfo("apply"))
	if err == nil {
		t.Fatal("run B should be blocked while run A holds the lock")
	}
	if !strings.Contains(err.Error(), infoA.Who) || !strings.Contains(err.Error(), "force-unlock") {
		t.Errorf("blocked error %q should name the holder %q and suggest force-unlock", err.Error(), infoA.Who)
	}

	// A releases; B can now acquire.
	if err := runA.Unlock(idA); err != nil {
		t.Fatalf("run A release: %v", err)
	}
	if _, err := runB.Lock(state.NewLockInfo("apply")); err != nil {
		t.Fatalf("run B acquire after release: %v", err)
	}

	// force-unlock clears B's lock (the crashed-run escape hatch).
	if err := runB.ForceUnlock(); err != nil {
		t.Fatalf("force-unlock: %v", err)
	}
	if _, err := runA.Lock(state.NewLockInfo("apply")); err != nil {
		t.Fatalf("acquire after force-unlock: %v", err)
	}
}
