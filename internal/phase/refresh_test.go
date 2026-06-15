// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/wearetechnative/nivis/internal/ledger"
	"github.com/wearetechnative/nivis/internal/state"
)

// refreshDriver is a driver with refresh ON (the default), for testing that
// applyOne reads real state before planning (beans-q3ku).
func refreshDriver(t *testing.T, c *recordClient) *Driver {
	t.Helper()
	st, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	return &Driver{Manager: recordManager{c: c}, Store: st, Ledger: ledger.New()} // NoRefresh defaults false
}

// With refresh on, the prior state passed to Plan/Apply is the provider's READ
// result, not the stored attrs — so out-of-band drift is planned against.
func TestApplyOneRefreshUsesReadState(t *testing.T) {
	c := &recordClient{readAttrs: map[string]interface{}{"id": "drifted"}}
	d := refreshDriver(t, c)
	g, node := replaceGraphNode(nil)
	// Stored state says "old"; the real world (Read) says "drifted".
	if err := d.Store.Set(state.ResourceState{ID: node.Resource.ID, Type: node.Resource.Type, Attrs: map[string]interface{}{"id": "old"}}); err != nil {
		t.Fatal(err)
	}

	if _, err := d.applyOne(context.Background(), g, node, map[string]interface{}{}); err != nil {
		t.Fatal(err)
	}
	if c.readCalls != 1 {
		t.Errorf("refresh on must Read exactly once; readCalls=%d", c.readCalls)
	}
	// Update-in-place applies against the REFRESHED prior, not the stored one.
	if c.applyPrior["id"] != "drifted" {
		t.Errorf("apply must use the refreshed (read) prior; got %v", c.applyPrior)
	}
}

// A resource deleted out-of-band (Read returns empty) is re-created: no prior,
// no destroy, Apply runs as a create.
func TestApplyOneRefreshDeletedRecreates(t *testing.T) {
	c := &recordClient{readAttrs: map[string]interface{}{}} // empty read = gone
	d := refreshDriver(t, c)
	g, node := replaceGraphNode(nil)
	if err := d.Store.Set(state.ResourceState{ID: node.Resource.ID, Type: node.Resource.Type, Attrs: map[string]interface{}{"id": "old"}}); err != nil {
		t.Fatal(err)
	}

	if _, err := d.applyOne(context.Background(), g, node, map[string]interface{}{}); err != nil {
		t.Fatal(err)
	}
	if c.readCalls != 1 {
		t.Errorf("expected one Read; got %d", c.readCalls)
	}
	if c.destroyCalls != 0 {
		t.Errorf("a deleted resource must be re-created, not destroyed; destroyCalls=%d", c.destroyCalls)
	}
	if c.applyPrior != nil {
		t.Errorf("re-create must apply with nil prior (a create); got %v", c.applyPrior)
	}
}

// --refresh=false (NoRefresh) plans against stored state without reading.
func TestApplyOneNoRefreshSkipsRead(t *testing.T) {
	c := &recordClient{readAttrs: map[string]interface{}{"id": "drifted"}}
	st, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	d := &Driver{Manager: recordManager{c: c}, Store: st, Ledger: ledger.New(), NoRefresh: true}
	g, node := replaceGraphNode(nil)
	if err := d.Store.Set(state.ResourceState{ID: node.Resource.ID, Type: node.Resource.Type, Attrs: map[string]interface{}{"id": "old"}}); err != nil {
		t.Fatal(err)
	}

	if _, err := d.applyOne(context.Background(), g, node, map[string]interface{}{}); err != nil {
		t.Fatal(err)
	}
	if c.readCalls != 0 {
		t.Errorf("NoRefresh must NOT Read; readCalls=%d", c.readCalls)
	}
	if c.applyPrior["id"] != "old" {
		t.Errorf("NoRefresh must use the STORED prior; got %v", c.applyPrior)
	}
}
