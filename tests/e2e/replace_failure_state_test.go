// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package e2e_test

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/nivis-project/nivis/internal/ledger"
	"github.com/nivis-project/nivis/internal/phase"
	"github.com/nivis-project/nivis/internal/plan"
	"github.com/nivis-project/nivis/internal/plugin"
	"github.com/nivis-project/nivis/internal/state"
)

// These tests drive the REAL provider-delta binary and a REAL on-disk state
// store, which are the two things the invariant is about: a state document must
// not describe an object the executor has destroyed.
//
// The IR comes from a StubEvaluator rather than the Nix topology, because no
// flake attribute declares a force-new attribute and inventing a public one for
// a failure-path test would be a larger change than the fix. Nix evaluation is
// not part of what this invariant depends on.

// buildDelta builds the real provider-delta binary and returns its path. It
// SKIPS rather than fails when the build is unavailable, matching buildBinaries,
// so an environment that cannot build a sub-binary does not turn into a red
// suite. These tests need no Nix, so they do not call requireNix.
func buildDelta(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "provider-delta")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/provider-delta")
	cmd.Dir = repoRoot(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot build provider-delta: %v\n%s", err, out)
	}
	return bin
}

func deltaIR(bin, image string) func(*ledger.Ledger) []byte {
	return func(_ *ledger.Ledger) []byte {
		return []byte(fmt.Sprintf(`{
		  "schemaVersion":1,
		  "providers":{"delta":{"source":%q,"config":{}}},
		  "resources":[
		    {"id":"delta.delta_thing.t","provider":"delta","type":"delta_thing","name":"t",
		     "config":{"label":"L","image":%q}}
		  ],
		  "dataSources":[],
		  "edges":[],
		  "nixConsumers":[]
		}`, bin, image))
	}
}

func deltaDriver(bin, image, statePath string, noRefresh bool) (*phase.Driver, func(), error) {
	st, err := state.Open(statePath)
	if err != nil {
		return nil, func() {}, err
	}
	mgr := plugin.NewManager()
	d := &phase.Driver{
		Eval:      &phase.StubEvaluator{IRForLedger: deltaIR(bin, image)},
		Manager:   mgr,
		Store:     st,
		Ledger:    ledger.New(),
		MaxPhases: 10,
		NoRefresh: noRefresh,
	}
	return d, mgr.Close, nil
}

// TestFailedReplacementLeavesNoStaleStateEntry is the regression for bean
// nixform2-sw6c. The prior object is destroyed and the create then fails; state
// must not keep describing the destroyed object.
func TestFailedReplacementLeavesNoStaleStateEntry(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	bin := buildDelta(t)
	ctx := context.Background()
	statePath := t.TempDir() + "/state.json"

	// 1. Create it, so there is a prior object to replace.
	d, closeMgr, err := deltaDriver(bin, "ami-old", statePath, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Run(ctx); err != nil {
		closeMgr()
		t.Fatalf("initial apply: %v", err)
	}
	closeMgr()

	st, _ := state.Open(statePath)
	if _, found, _ := st.Get("delta.delta_thing.t"); !found {
		t.Fatalf("expected the resource in state after the first apply")
	}

	// 2. Change the force-new attribute so it is replaced, with the provider
	//    told to fail the create that follows the destroy.
	t.Setenv("TERRAE_NIVIS_FAKE_FAIL", "apply:delta_thing")
	d2, closeMgr2, err := deltaDriver(bin, "ami-new", statePath, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = d2.Run(ctx)
	closeMgr2()
	if err == nil {
		t.Fatalf("expected the replacement to fail; the injected apply failure did not take effect")
	}

	// 3. The prior object is gone, so state must not claim it exists.
	st2, _ := state.Open(statePath)
	if rs, found, _ := st2.Get("delta.delta_thing.t"); found {
		t.Errorf("state still describes the destroyed resource: %#v", rs.Attrs)
	}
}

// TestStaleEntryWouldHideAMissingResource is the discriminating half, and it
// models the actual harm rather than the mechanism.
//
// After a failed replacement the object is gone. If the user then reverts the
// configuration, a stale state entry makes the next plan a NO-OP: prior and
// config agree, so nivis reports the resource is up to date and creates
// nothing, while the object does not exist. Refresh is disabled, because that
// is the configuration in which nothing heals it.
//
// Re-running with the NEW config would not discriminate: a stale entry still
// plans as a replacement and recreates the object either way.
func TestStaleEntryWouldHideAMissingResource(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	bin := buildDelta(t)
	ctx := context.Background()
	statePath := t.TempDir() + "/state.json"

	// 1. Create with ami-old.
	d, closeMgr, err := deltaDriver(bin, "ami-old", statePath, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Run(ctx); err != nil {
		closeMgr()
		t.Fatalf("initial apply: %v", err)
	}
	closeMgr()

	// 2. Replace to ami-new, with the create failing after the destroy.
	t.Setenv("TERRAE_NIVIS_FAKE_FAIL", "apply:delta_thing")
	d2, closeMgr2, err := deltaDriver(bin, "ami-new", statePath, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d2.Run(ctx); err == nil {
		closeMgr2()
		t.Fatalf("expected the replacement to fail")
	}
	closeMgr2()

	// 3. The user reverts to ami-old. A stale entry would match this config
	//    exactly, so the run would do nothing and the object would stay missing.
	t.Setenv("TERRAE_NIVIS_FAKE_FAIL", "")
	d3, closeMgr3, err := deltaDriver(bin, "ami-old", statePath, true)
	if err != nil {
		t.Fatal(err)
	}
	res, err := d3.Run(ctx)
	closeMgr3()
	if err != nil {
		t.Fatalf("recovery run with refresh disabled: %v", err)
	}
	// Assert the OPERATION, not res.Applied: the phase loop records every node it
	// resolves, no-ops included, so "applied" there does not mean "created".
	var op plan.Op = -1
	for _, ph := range res.Phases {
		for _, n := range ph {
			if n.ID == "delta.delta_thing.t" {
				op = n.Op
			}
		}
	}
	if op != plan.OpCreate {
		t.Fatalf("the resource resolved as %v, want a create. A stale state entry matched the "+
			"reverted config, so the run reported no change while the object does not exist.", op)
	}

	st, _ := state.Open(statePath)
	rs, found, _ := st.Get("delta.delta_thing.t")
	if !found {
		t.Fatalf("expected the resource in state after recovery")
	}
	if rs.Attrs["image"] != "ami-old" {
		t.Errorf("image = %v after recovery, want ami-old", rs.Attrs["image"])
	}
}

// A successful replacement is unchanged: state ends with the new attributes.
func TestSuccessfulReplacementStillWritesState(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	bin := buildDelta(t)
	ctx := context.Background()
	statePath := t.TempDir() + "/state.json"

	d, closeMgr, err := deltaDriver(bin, "ami-old", statePath, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Run(ctx); err != nil {
		closeMgr()
		t.Fatalf("initial apply: %v", err)
	}
	closeMgr()

	d2, closeMgr2, err := deltaDriver(bin, "ami-new", statePath, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = d2.Run(ctx)
	closeMgr2()
	if err != nil {
		t.Fatalf("replacement: %v", err)
	}

	st, _ := state.Open(statePath)
	rs, found, _ := st.Get("delta.delta_thing.t")
	if !found {
		t.Fatalf("expected the resource in state after a successful replacement")
	}
	if rs.Attrs["image"] != "ami-new" {
		t.Errorf("image = %v, want ami-new", rs.Attrs["image"])
	}
}
