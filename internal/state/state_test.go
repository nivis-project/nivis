// Copyright 2026 WeareTechnative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/wearetechnative/nivis/internal/state"
)

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	st, err := state.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	rs := state.ResourceState{
		ID:    "alpha.alpha_token.A",
		Type:  "alpha_token",
		Attrs: map[string]interface{}{"id": "alpha-0", "value": "alpha:seed:0"},
	}
	if err := st.Set(rs); err != nil {
		t.Fatal(err)
	}

	// Reopen from disk to prove persistence.
	st2, err := state.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := st2.Get("alpha.alpha_token.A")
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if got.Attrs["value"] != "alpha:seed:0" {
		t.Errorf("value = %v, want alpha:seed:0", got.Attrs["value"])
	}
}

func TestListAndDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	st, _ := state.Open(path)
	for _, id := range []string{"a.t.C", "a.t.A", "a.t.B"} {
		if err := st.Set(state.ResourceState{ID: id, Type: "t", Attrs: map[string]interface{}{}}); err != nil {
			t.Fatal(err)
		}
	}
	list, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[0].ID != "a.t.A" || list[2].ID != "a.t.C" {
		t.Fatalf("list not sorted/complete: %+v", list)
	}
	if err := st.Delete("a.t.B"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := st.Get("a.t.B"); ok {
		t.Error("B should be deleted")
	}
}

func TestConcurrentWritersSerialized(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	st, _ := state.Open(path)

	const n = 20
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := "a.t." + string(rune('A'+i))
			errs <- st.Set(state.ResourceState{ID: id, Type: "t", Attrs: map[string]interface{}{"i": i}})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Set: %v", err)
		}
	}
	// All n writes must survive (no lost updates under the lock).
	list, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != n {
		t.Fatalf("expected %d resources after concurrent writes, got %d", n, len(list))
	}
}

func TestPartialStateSurvives(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	st, _ := state.Open(path)
	// A is persisted; "process exits" before B; reopening still has A.
	if err := st.Set(state.ResourceState{ID: "a.t.A", Type: "t", Attrs: map[string]interface{}{"v": 1}}); err != nil {
		t.Fatal(err)
	}
	reopened, _ := state.Open(path)
	if _, ok, _ := reopened.Get("a.t.A"); !ok {
		t.Fatal("A's persisted state should survive a restart")
	}
}
