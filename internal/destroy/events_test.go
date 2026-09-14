// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package destroy_test

import (
	"context"
	"testing"

	"github.com/nivis-project/nivis/internal/destroy"
	"github.com/nivis-project/nivis/internal/plugin"
	"github.com/nivis-project/nivis/internal/progress"
)

type rec struct{ events []progress.Event }

func (r *rec) Emit(e progress.Event) { r.events = append(r.events, e) }

// A destroy reports each resource as it goes. Before this it produced nothing
// until every provider round trip had completed.
func TestDestroyReportsEachNode(t *testing.T) {
	bin := buildAlpha(t)
	g := graphAB(t, bin, false)
	st, _ := seedState(t)
	mgr := plugin.NewManager()
	defer mgr.Close()

	r := &rec{}
	res, err := destroy.Run(context.Background(), g, mgr, st, destroy.Options{Observer: r})
	if err != nil {
		t.Fatalf("destroy: %v", err)
	}

	var done []progress.Event
	for _, e := range r.events {
		if e.Kind == progress.NodeDone {
			done = append(done, e)
		}
	}
	if len(done) != len(res.Destroyed) {
		t.Fatalf("done events = %d, want one per destroyed resource (%d)",
			len(done), len(res.Destroyed))
	}
	// Reported in the order they were destroyed, so a reader sees teardown order.
	for i, e := range done {
		if e.ID != res.Destroyed[i] {
			t.Errorf("event %d is %q, want %q (destroy order must be preserved)",
				i, e.ID, res.Destroyed[i])
		}
		if e.Err != nil {
			t.Errorf("%s reported an error on a successful destroy: %v", e.ID, e.Err)
		}
	}
}

// A nil observer discards rather than panicking.
func TestDestroyWithoutObserver(t *testing.T) {
	bin := buildAlpha(t)
	g := graphAB(t, bin, false)
	st, _ := seedState(t)
	mgr := plugin.NewManager()
	defer mgr.Close()
	if _, err := destroy.Run(context.Background(), g, mgr, st, destroy.Options{}); err != nil {
		t.Fatalf("destroy: %v", err)
	}
}
