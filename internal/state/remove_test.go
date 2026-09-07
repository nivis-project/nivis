// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state_test

// The whole-document removal seam: the local store clears its file and derived
// siblings, the S3 store deletes only its own object, and both are idempotent.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nivis-project/nivis/internal/fakes3"
	"github.com/nivis-project/nivis/internal/state"
)

// asRemover asserts the store offers whole-document removal.
func asRemover(t *testing.T, st state.Store) state.Remover {
	t.Helper()
	r, ok := st.(state.Remover)
	if !ok {
		t.Fatalf("%T does not implement state.Remover", st)
	}
	return r
}

// The local store removes the state file plus its ledger and lock siblings, and a
// store reopened at that path reads empty.
func TestFileStoreRemoveClearsDocumentAndSiblings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nivis.state.json")
	st, err := state.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := st.Set(state.ResourceState{ID: "alpha.alpha_token.app", Type: "alpha_token", Attrs: map[string]interface{}{"id": "t1"}}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	// The ledger sidecar the phase driver writes next to the state file.
	ledger := path + ".ledger"
	if err := os.WriteFile(ledger, []byte(`{"outputs":{}}`), 0o600); err != nil {
		t.Fatalf("write ledger: %v", err)
	}

	if err := asRemover(t, st).Remove(); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	for _, p := range []string{path, ledger, path + ".lock"} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s still exists after Remove (err = %v)", filepath.Base(p), err)
		}
	}

	// A store reopened at the same path sees a fresh, empty state.
	st2, err := state.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	list, err := st2.List()
	if err != nil || len(list) != 0 {
		t.Errorf("reopened store = (%v, %v), want empty with no error", list, err)
	}
}

// Removing an absent local document succeeds (idempotence), including twice.
func TestFileStoreRemoveIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.json")
	st, err := state.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	r := asRemover(t, st)
	if err := r.Remove(); err != nil {
		t.Errorf("Remove of an absent document: %v", err)
	}
	if err := r.Remove(); err != nil {
		t.Errorf("second Remove: %v", err)
	}
}

// The S3 store deletes its own object only: the bucket and unrelated keys survive.
func TestS3StoreRemoveDeletesOnlyItsObject(t *testing.T) {
	srv := fakes3.NewWithBuckets("nivis-state")
	defer srv.Close()

	st := openS3(t, srv, "nivis-state", "prod/app.json")
	other := openS3(t, srv, "nivis-state", "staging/app.json")
	if err := st.Set(state.ResourceState{ID: "a", Type: "alpha_token", Attrs: map[string]interface{}{"id": "t1"}}); err != nil {
		t.Fatalf("Set prod: %v", err)
	}
	if err := other.Set(state.ResourceState{ID: "b", Type: "alpha_token", Attrs: map[string]interface{}{"id": "t2"}}); err != nil {
		t.Fatalf("Set staging: %v", err)
	}

	if err := asRemover(t, st).Remove(); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if srv.Has("nivis-state", "prod/app.json") {
		t.Error("the state object was not deleted")
	}
	if !srv.Has("nivis-state", "staging/app.json") {
		t.Error("Remove deleted an unrelated key in the same bucket")
	}
	// The bucket still exists: a fresh write to it works.
	if err := st.Set(state.ResourceState{ID: "c", Type: "alpha_token", Attrs: map[string]interface{}{"id": "t3"}}); err != nil {
		t.Errorf("bucket unusable after Remove: %v", err)
	}
}

// The S3 store's removal is idempotent, and a fresh store over the removed key
// reads as an empty stack.
func TestS3StoreRemoveIsIdempotent(t *testing.T) {
	srv := fakes3.NewWithBuckets("nivis-state")
	defer srv.Close()
	st := openS3(t, srv, "nivis-state", "prod/app.json")
	r := asRemover(t, st)

	if err := r.Remove(); err != nil {
		t.Errorf("Remove of an absent object: %v", err)
	}
	if err := st.Set(state.ResourceState{ID: "a", Type: "alpha_token", Attrs: map[string]interface{}{"id": "t1"}}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := r.Remove(); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := r.Remove(); err != nil {
		t.Errorf("second Remove: %v", err)
	}
	list, err := st.List()
	if err != nil || len(list) != 0 {
		t.Errorf("store after Remove = (%v, %v), want empty with no error", list, err)
	}
}

// The lock object is NOT collateral damage: a migration holds the destination lock
// while removing a document, so removal must leave the lock object alone.
func TestS3StoreRemoveKeepsTheLockObject(t *testing.T) {
	srv := fakes3.NewWithBuckets("nivis-state")
	defer srv.Close()
	st := openS3(t, srv, "nivis-state", "prod/app.json")

	lk, ok := st.(state.Locker)
	if !ok {
		t.Fatal("the s3 store should implement state.Locker")
	}
	id, err := lk.Lock(state.NewLockInfo("state migrate"))
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if err := asRemover(t, st).Remove(); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !srv.Has("nivis-state", "prod/app.json.lock") {
		t.Error("Remove deleted the lock object that the caller still holds")
	}
	if err := lk.Unlock(id); err != nil {
		t.Errorf("Unlock after Remove: %v", err)
	}
}
