// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/nivis-project/nivis/internal/state"
)

func TestSnapshotRoundTripsThroughRestore(t *testing.T) {
	src, _ := state.Open(filepath.Join(t.TempDir(), "src.json"))
	for _, id := range []string{"a.b.c", "a.b.d"} {
		if err := src.Set(state.ResourceState{ID: id, Type: "t", Attrs: map[string]interface{}{"k": id}}); err != nil {
			t.Fatal(err)
		}
	}
	snap, err := src.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	dst, _ := state.Open(filepath.Join(t.TempDir(), "dst.json"))
	if err := dst.Restore(snap); err != nil {
		t.Fatal(err)
	}
	got, _ := dst.List()
	if len(got) != 2 || got[0].ID != "a.b.c" || got[1].ID != "a.b.d" {
		t.Errorf("restored list = %v, want [a.b.c a.b.d]", got)
	}
	if got[0].Attrs["k"] != "a.b.c" {
		t.Errorf("attrs lost in round trip: %v", got[0].Attrs)
	}
}

func TestRestoreReplacesWholeDocument(t *testing.T) {
	st, _ := state.Open(filepath.Join(t.TempDir(), "s.json"))
	if err := st.Set(state.ResourceState{ID: "old.x.y", Type: "t", Attrs: map[string]interface{}{}}); err != nil {
		t.Fatal(err)
	}
	// restore a document containing only a different resource
	doc := `{"resources":{"new.a.b":{"id":"new.a.b","type":"t","attrs":{}}}}`
	if err := st.Restore([]byte(doc)); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := st.Get("old.x.y"); ok {
		t.Error("old resource should be gone after restore")
	}
	if _, ok, _ := st.Get("new.a.b"); !ok {
		t.Error("new resource should be present after restore")
	}
}

func TestRestoreRejectsMalformedLeavingStateUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.json")
	st, _ := state.Open(path)
	if err := st.Set(state.ResourceState{ID: "keep.me.here", Type: "t", Attrs: map[string]interface{}{}}); err != nil {
		t.Fatal(err)
	}
	if err := st.Restore([]byte("not json at all")); err == nil {
		t.Fatal("expected restore of malformed input to error")
	}
	// existing state untouched
	if _, ok, _ := st.Get("keep.me.here"); !ok {
		t.Error("existing state must survive a rejected restore")
	}
}

// A held lock makes a state op fail with the actionable timeout error, not hang.
func TestLockTimeoutActionableError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.json")
	// a short lock timeout keeps the test fast (the default would wait seconds).
	st, _ := state.OpenWithLockTimeout(path, 200*time.Millisecond)
	// seed the file so the lock path exists alongside it
	if err := st.Set(state.ResourceState{ID: "a.b.c", Type: "t", Attrs: map[string]interface{}{}}); err != nil {
		t.Fatal(err)
	}

	// hold the advisory lock from the test, the way another process would.
	lf, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lf.Close()
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	defer syscall.Flock(int(lf.Fd()), syscall.LOCK_UN)

	// Use a short-timeout store so the test is fast (a default-timeout store would
	// also work but wait the full default). state.Open uses the default; to keep
	// the test quick we rely on the default being bounded and assert the error.
	done := make(chan error, 1)
	go func() {
		_, e := st.List()
		done <- e
	}()
	// List() should return (with the lock-timeout error) within the default window.
	err = <-done
	if err == nil {
		t.Fatal("expected a lock-timeout error while the lock is held")
	}
	if !strings.Contains(err.Error(), "locked") || !strings.Contains(err.Error(), ".lock") {
		t.Errorf("error should name the lock and say locked; got %v", err)
	}
}
