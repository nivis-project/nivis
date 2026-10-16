// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/ledger"
	"github.com/nivis-project/nivis/internal/phase"
	"github.com/nivis-project/nivis/internal/plan"
	"github.com/nivis-project/nivis/internal/plugin"
	"github.com/nivis-project/nivis/internal/progress"
	"github.com/nivis-project/nivis/internal/state"
)

// A three-layer chain whose references land in `image`, which delta_thing
// declares ForceNew. That is what makes the cascade reachable: replacing a
// resource makes its dependent's `image` unknown, and an unknown in a ForceNew
// attribute is a replacement.
//
// The existing chain in held_replace_test.go references through `label`, which
// is NOT ForceNew, so it exercises the opposite case: a holder that updates in
// place and still blocks. Both shapes are real, and the pair is the whole point
// of asking the provider rather than assuming (bean nixform2-nwnf, where this
// was measured against cyrilgdn/postgresql: an unknown in an identity-bearing
// attribute replaced, an unknown in `owner` did not).
func cascadeIR(bin, bottomImage string) func(*ledger.Ledger) []byte {
	return func(_ *ledger.Ledger) []byte {
		return []byte(fmt.Sprintf(`{
		  "schemaVersion":1,
		  "providers":{"delta":{"source":%q,"config":{}}},
		  "resources":[
		    {"id":"delta.delta_thing.bottom","provider":"delta","type":"delta_thing","name":"bottom",
		     "config":{"label":"b","image":%q}},
		    {"id":"delta.delta_thing.middle","provider":"delta","type":"delta_thing","name":"middle",
		     "config":{"label":"m","image":{"__ref":{"resource":"delta.delta_thing.bottom","path":["id"]}}}},
		    {"id":"delta.delta_thing.top","provider":"delta","type":"delta_thing","name":"top",
		     "config":{"label":"t","image":{"__ref":{"resource":"delta.delta_thing.middle","path":["id"]}}}}
		  ],
		  "dataSources":[],
		  "edges":[],
		  "nixConsumers":[]
		}`, bin, bottomImage))
	}
}

func cascadeDriver(t *testing.T, bin, bottomImage, statePath string) (*phase.Driver, func()) {
	t.Helper()
	st, err := state.Open(statePath)
	if err != nil {
		t.Fatal(err)
	}
	mgr := plugin.NewManager()
	return &phase.Driver{
		Eval:      &phase.StubEvaluator{IRForLedger: cascadeIR(bin, bottomImage)},
		Manager:   mgr,
		Store:     st,
		Ledger:    ledger.New(),
		MaxPhases: 10,
	}, mgr.Close
}

// applyCascadeChain brings the chain up and returns the state path plus each
// resource's id, so a later assertion can tell a replacement from a no-op.
func applyCascadeChain(t *testing.T, bin string) (string, map[string]interface{}) {
	t.Helper()
	statePath := t.TempDir() + "/state.json"
	d, closeMgr := cascadeDriver(t, bin, "img-old", statePath)
	if _, err := d.Run(context.Background()); err != nil {
		closeMgr()
		t.Fatalf("initial apply: %v", err)
	}
	closeMgr()

	st, _ := state.Open(statePath)
	ids := map[string]interface{}{}
	for _, id := range cascadeIDs {
		rs, found, _ := st.Get(id)
		if !found {
			t.Fatalf("%s missing from state after the first apply", id)
		}
		ids[id] = rs.Attrs["id"]
	}
	return statePath, ids
}

var cascadeIDs = []string{
	"delta.delta_thing.bottom",
	"delta.delta_thing.middle",
	"delta.delta_thing.top",
}

// The headline: replacing the bottom of a chain carries through to everything
// that reads it through a ForceNew attribute, instead of being refused.
//
// Asserted on the DESTROYS the provider was asked for, not on the resource ids:
// the fake's counter restarts with each provider process, so a recreated
// resource gets the same id it had and "the id changed" would prove nothing.
func TestReplacementCascadesThroughForceNewDependents(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	bin := buildProvider(t, "provider-delta")
	statePath, _ := applyCascadeChain(t, bin)

	obs := &destroyOrderObserver{}
	d, closeMgr := cascadeDriver(t, bin, "img-new", statePath)
	d.Observer = obs
	res, err := d.Run(context.Background())
	closeMgr()
	if err != nil {
		t.Fatalf("the cascade was refused: %v", err)
	}

	for _, id := range cascadeIDs {
		if !contains(obs.destroyed, id) {
			t.Errorf("%s was not destroyed, so the cascade did not reach it (destroyed: %v)",
				id, obs.destroyed)
		}
	}

	// And each is reported as a replacement rather than a create, since a create
	// would hide the destroy that happened.
	ops := appliedOps(res)
	for _, id := range cascadeIDs {
		if ops[id] != plan.OpReplace {
			t.Errorf("%s reported as %v, want replace", id, ops[id])
		}
	}

	st, _ := state.Open(statePath)
	bottom, found, _ := st.Get("delta.delta_thing.bottom")
	if !found || bottom.Attrs["image"] != "img-new" {
		t.Errorf("bottom was not recreated with the new image: %#v", bottom.Attrs)
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func appliedOps(res *phase.Result) map[string]plan.Op {
	ops := map[string]plan.Op{}
	if res == nil {
		return ops
	}
	for _, group := range res.Phases {
		for _, n := range group {
			ops[n.ID] = n.Op
		}
	}
	return ops
}

// Transitivity. `top` reads `middle`, which reads `bottom`: the set must reach
// it even though nothing about `top` references `bottom` directly.
func TestCascadeIsTransitive(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	bin := buildProvider(t, "provider-delta")
	statePath, _ := applyCascadeChain(t, bin)

	obs := &destroyOrderObserver{}
	d, closeMgr := cascadeDriver(t, bin, "img-new", statePath)
	d.Observer = obs
	_, err := d.Run(context.Background())
	closeMgr()
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if !contains(obs.destroyed, "delta.delta_thing.top") {
		t.Errorf("top was not destroyed, so the set did not reach two links out: %v", obs.destroyed)
	}
}

// A holder the provider does NOT plan as a replacement still blocks, and the
// message says what that case needs. This is the `label` chain, measured as
// real behaviour: an unknown in an attribute a provider can update in place
// does not force a new object.
func TestHolderThatUpdatesInPlaceStillRefuses(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	bin := buildProvider(t, "provider-delta")
	statePath := applyChain(t, bin)

	before, _ := state.Open(statePath)
	mid, _, _ := before.Get("delta.delta_thing.middle")

	d, closeMgr := chainDriver(t, bin, "ami-new", statePath)
	_, err := d.Run(context.Background())
	closeMgr()
	if err == nil {
		t.Fatalf("expected the replacement to be refused while top holds middle in place")
	}
	if !strings.Contains(err.Error(), "delta.delta_thing.top") {
		t.Errorf("the refusal does not name the holder: %v", err)
	}

	after, _ := state.Open(statePath)
	got, found, _ := after.Get("delta.delta_thing.middle")
	if !found || got.Attrs["id"] != mid.Attrs["id"] {
		t.Errorf("middle was touched despite the refusal: %#v", got.Attrs)
	}
}

// The destroy half runs in reverse dependency order. This is the whole reason
// the prologue exists: a backing service refuses to destroy an object another
// object holds, and answers in its own vocabulary, late, after other resources
// in the run have already changed.
//
// Falsified by building the loop on DAG.ApplyOrder instead of DestroyOrder:
// the order below then comes back bottom, middle, top and the test fails.
func TestCascadeDestroysHoldersFirst(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	bin := buildProvider(t, "provider-delta")
	statePath, _ := applyCascadeChain(t, bin)

	obs := &destroyOrderObserver{}
	d, closeMgr := cascadeDriver(t, bin, "img-new", statePath)
	d.Observer = obs
	_, err := d.Run(context.Background())
	closeMgr()
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	want := []string{
		"delta.delta_thing.top",
		"delta.delta_thing.middle",
		"delta.delta_thing.bottom",
	}
	if len(obs.destroyed) != len(want) {
		t.Fatalf("destroyed %v, want all three in reverse dependency order", obs.destroyed)
	}
	for i := range want {
		if obs.destroyed[i] != want[i] {
			t.Fatalf("destroy order %v, want %v: a holder was destroyed after the object it holds",
				obs.destroyed, want)
		}
	}
}

// destroyOrderObserver records the ids destroyed as the replacement half, in
// the order the provider was asked.
type destroyOrderObserver struct{ destroyed []string }

func (o *destroyOrderObserver) Emit(e progress.Event) {
	if e.Kind == progress.NodeStart && e.Destroy {
		o.destroyed = append(o.destroyed, e.ID)
	}
}

// Computing the set must change nothing. A run whose configuration is unchanged
// touches no provider beyond reading, which is what makes the prologue safe to
// run at the top of every phase.
func TestComputingTheSetChangesNothing(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	bin := buildProvider(t, "provider-delta")
	statePath, before := applyCascadeChain(t, bin)

	d, closeMgr := cascadeDriver(t, bin, "img-old", statePath)
	_, err := d.Run(context.Background())
	closeMgr()
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	st, _ := state.Open(statePath)
	for _, id := range cascadeIDs {
		rs, found, _ := st.Get(id)
		if !found {
			t.Fatalf("%s vanished from an unchanged run", id)
		}
		if rs.Attrs["id"] != before[id] {
			t.Errorf("%s id %v -> %v: an unchanged run replaced it", id, before[id], rs.Attrs["id"])
		}
	}
}

// The plan reports the whole set before anything runs, and stays side-effect
// free. A cascade is a larger blast radius than the resource whose
// configuration changed, so seeing only that resource would understate it.
func TestPlanReportsTheWholeCascade(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	bin := buildProvider(t, "provider-delta")
	statePath, before := applyCascadeChain(t, bin)

	d, closeMgr := cascadeDriver(t, bin, "img-new", statePath)
	items, err := d.PlanReport(context.Background())
	closeMgr()
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	ops := map[string]plan.Op{}
	for _, it := range items {
		ops[it.ID] = it.Op
	}
	for _, id := range cascadeIDs {
		if ops[id] != plan.OpReplace {
			t.Errorf("plan reports %s as op %v, want replace", id, ops[id])
		}
	}

	st, _ := state.Open(statePath)
	for _, id := range cascadeIDs {
		rs, found, _ := st.Get(id)
		if !found || rs.Attrs["id"] != before[id] {
			t.Errorf("planning changed %s", id)
		}
	}
}

// Only resources whose configuration is already resolved may join the set.
//
// An unknown produced by withholding a member's outputs and an unknown left by
// a reference this phase has not resolved look identical once encoded, so a
// resource waiting on something unrelated would otherwise be planned with an
// unknown it never asked for and pulled into a cascade that is not replacing
// anything it reads. Here `spare` waits on a Nix-computed value derived from
// `anchor`, and reads nothing in the chain being replaced.
//
// Falsified by seeding the candidates from every in-state resource rather than
// from the resolved ones: `spare` is then planned with an unknown `image`,
// joins the set and is destroyed, and the assertion below fails.
func TestUnresolvedResourceDoesNotJoinTheSet(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	bin := buildProvider(t, "provider-delta")
	statePath := t.TempDir() + "/state.json"

	irFn := func(image string) func(*ledger.Ledger) []byte {
		return func(l *ledger.Ledger) []byte {
			spareImage := `{"__derived":{"inputs":["delta.delta_thing.anchor.id"]}}`
			if outs, ok := l.Outputs["delta.delta_thing.anchor"]; ok {
				spareImage = fmt.Sprintf("%q", fmt.Sprintf("spare-%v", outs["id"]))
			}
			return []byte(fmt.Sprintf(`{
			  "schemaVersion":1,
			  "providers":{"delta":{"source":%q,"config":{}}},
			  "resources":[
			    {"id":"delta.delta_thing.bottom","provider":"delta","type":"delta_thing","name":"bottom",
			     "config":{"label":"b","image":%q}},
			    {"id":"delta.delta_thing.mid","provider":"delta","type":"delta_thing","name":"mid",
			     "config":{"label":"m","image":{"__ref":{"resource":"delta.delta_thing.bottom","path":["id"]}}}},
			    {"id":"delta.delta_thing.anchor","provider":"delta","type":"delta_thing","name":"anchor",
			     "config":{"label":"a","image":"anchor"}},
			    {"id":"delta.delta_thing.spare","provider":"delta","type":"delta_thing","name":"spare",
			     "config":{"label":"s","image":%s}}
			  ],
			  "dataSources":[],
			  "edges":[],
			  "nixConsumers":[]
			}`, bin, image, spareImage))
		}
	}

	if _, err := runCascade(t, statePath, irFn("img-old")); err != nil {
		t.Fatalf("initial apply: %v", err)
	}
	obs, err := runCascade(t, statePath, irFn("img-new"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if !contains(obs.destroyed, "delta.delta_thing.mid") {
		t.Fatalf("mid did not cascade, so this test is not exercising a set: %v", obs.destroyed)
	}
	if contains(obs.destroyed, "delta.delta_thing.spare") {
		t.Errorf("spare joined the replacement set, but it reads nothing being replaced: %v", obs.destroyed)
	}
}

// A cascade among resources that only appear in the IR from a later phase.
// This is why the prologue runs once per phase: at phase 0 neither `late` nor
// `later` can be assessed, so a prologue that ran once would miss the cascade
// entirely and the per-node path would refuse it.
func TestALaterPhaseCascadeIsStillReached(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	bin := buildProvider(t, "provider-delta")
	statePath := t.TempDir() + "/state.json"

	// `late` takes its ForceNew image from a value computed in Nix out of
	// `anchor`'s output and a token that changes between runs, so it is
	// invisible until anchor is in the ledger. `later` reads late's id.
	irFn := func(token string) func(*ledger.Ledger) []byte {
		return func(l *ledger.Ledger) []byte {
			lateImage := `{"__derived":{"inputs":["delta.delta_thing.anchor.id"]}}`
			if outs, ok := l.Outputs["delta.delta_thing.anchor"]; ok {
				lateImage = fmt.Sprintf("%q", fmt.Sprintf("late-%v-%s", outs["id"], token))
			}
			return []byte(fmt.Sprintf(`{
			  "schemaVersion":1,
			  "providers":{"delta":{"source":%q,"config":{}}},
			  "resources":[
			    {"id":"delta.delta_thing.anchor","provider":"delta","type":"delta_thing","name":"anchor",
			     "config":{"label":"a","image":"anchor"}},
			    {"id":"delta.delta_thing.late","provider":"delta","type":"delta_thing","name":"late",
			     "config":{"label":"l","image":%s}},
			    {"id":"delta.delta_thing.later","provider":"delta","type":"delta_thing","name":"later",
			     "config":{"label":"r","image":{"__ref":{"resource":"delta.delta_thing.late","path":["id"]}}}}
			  ],
			  "dataSources":[],
			  "edges":[],
			  "nixConsumers":[]
			}`, bin, lateImage))
		}
	}

	if _, err := runCascade(t, statePath, irFn("t1")); err != nil {
		t.Fatalf("initial apply: %v", err)
	}
	st, _ := state.Open(statePath)
	for _, id := range []string{"delta.delta_thing.late", "delta.delta_thing.later"} {
		if _, found, _ := st.Get(id); !found {
			t.Fatalf("%s is not in state, so the second run cannot exercise the case", id)
		}
	}

	obs, err := runCascade(t, statePath, irFn("t2"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, id := range []string{"delta.delta_thing.late", "delta.delta_thing.later"} {
		if !contains(obs.destroyed, id) {
			t.Errorf("%s was not replaced: a cascade among later-phase resources was missed (destroyed: %v)",
				id, obs.destroyed)
		}
	}
	if contains(obs.destroyed, "delta.delta_thing.anchor") {
		t.Errorf("anchor was replaced, but nothing about it changed: %v", obs.destroyed)
	}
}

// runCascade applies one IR against an existing state file, recording what the
// provider was asked to destroy.
func runCascade(t *testing.T, statePath string, irFn func(*ledger.Ledger) []byte) (*destroyOrderObserver, error) {
	t.Helper()
	st, err := state.Open(statePath)
	if err != nil {
		t.Fatal(err)
	}
	mgr := plugin.NewManager()
	defer mgr.Close()
	obs := &destroyOrderObserver{}
	d := &phase.Driver{
		Eval:    &phase.StubEvaluator{IRForLedger: irFn},
		Manager: mgr, Store: st, Ledger: ledger.New(), MaxPhases: 10, Observer: obs,
	}
	_, err = d.Run(context.Background())
	return obs, err
}

// A holder the provider plans as an in-place update blocks the whole set, and
// nothing is destroyed. `watcher` reads mid's id into `label`, which
// delta_thing does not declare ForceNew, so an unknown there is an update and
// the reference cannot survive the gap.
//
// Falsified by dropping the refuseUnjoinedHolders call: bottom and mid are then
// destroyed and the assertions below fail.
func TestSetRefusedWhenAHolderUpdatesInPlace(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	bin := buildProvider(t, "provider-delta")
	statePath := t.TempDir() + "/state.json"

	irFn := func(image string) func(*ledger.Ledger) []byte {
		return func(_ *ledger.Ledger) []byte {
			return []byte(fmt.Sprintf(`{
			  "schemaVersion":1,
			  "providers":{"delta":{"source":%q,"config":{}}},
			  "resources":[
			    {"id":"delta.delta_thing.bottom","provider":"delta","type":"delta_thing","name":"bottom",
			     "config":{"label":"b","image":%q}},
			    {"id":"delta.delta_thing.mid","provider":"delta","type":"delta_thing","name":"mid",
			     "config":{"label":"m","image":{"__ref":{"resource":"delta.delta_thing.bottom","path":["id"]}}}},
			    {"id":"delta.delta_thing.watcher","provider":"delta","type":"delta_thing","name":"watcher",
			     "config":{"image":"w","label":{"__ref":{"resource":"delta.delta_thing.mid","path":["id"]}}}}
			  ],
			  "dataSources":[],
			  "edges":[],
			  "nixConsumers":[]
			}`, bin, image))
		}
	}

	if _, err := runCascade(t, statePath, irFn("img-old")); err != nil {
		t.Fatalf("initial apply: %v", err)
	}
	before, _ := state.Open(statePath)
	wasBottom, _, _ := before.Get("delta.delta_thing.bottom")

	obs, err := runCascade(t, statePath, irFn("img-new"))
	if err == nil {
		t.Fatalf("expected the set to be refused while watcher holds mid in place")
	}
	for _, want := range []string{"delta.delta_thing.watcher", "in-place", "create-before-destroy"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q: %v", want, err)
		}
	}
	if len(obs.destroyed) != 0 {
		t.Errorf("the refusal destroyed %v; it must destroy nothing", obs.destroyed)
	}

	after, _ := state.Open(statePath)
	got, found, _ := after.Get("delta.delta_thing.bottom")
	if !found || got.Attrs["image"] != wasBottom.Attrs["image"] {
		t.Errorf("state changed despite the refusal: %#v", got.Attrs)
	}
}

// A holder whose own configuration waits on a value this phase has not
// resolved cannot be assessed, so the set is refused rather than guessed at.
// The message says that, because it is not a judgement about the provider: the
// question was never asked.
func TestSetRefusedWhenAHolderCannotBeAssessed(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	bin := buildProvider(t, "provider-delta")
	statePath := t.TempDir() + "/state.json"

	irFn := func(image string) func(*ledger.Ledger) []byte {
		return func(l *ledger.Ledger) []byte {
			spareImage := `{"__derived":{"inputs":["delta.delta_thing.bottom.id"]}}`
			if outs, ok := l.Outputs["delta.delta_thing.bottom"]; ok {
				spareImage = fmt.Sprintf("%q", fmt.Sprintf("spare-%v", outs["id"]))
			}
			return []byte(fmt.Sprintf(`{
			  "schemaVersion":1,
			  "providers":{"delta":{"source":%q,"config":{}}},
			  "resources":[
			    {"id":"delta.delta_thing.bottom","provider":"delta","type":"delta_thing","name":"bottom",
			     "config":{"label":"b","image":%q}},
			    {"id":"delta.delta_thing.mid","provider":"delta","type":"delta_thing","name":"mid",
			     "config":{"label":"m","image":{"__ref":{"resource":"delta.delta_thing.bottom","path":["id"]}}}},
			    {"id":"delta.delta_thing.spare","provider":"delta","type":"delta_thing","name":"spare",
			     "config":{"label":"s","image":%s}}
			  ],
			  "dataSources":[],
			  "edges":[],
			  "nixConsumers":[]
			}`, bin, image, spareImage))
		}
	}

	if _, err := runCascade(t, statePath, irFn("img-old")); err != nil {
		t.Fatalf("initial apply: %v", err)
	}
	obs, err := runCascade(t, statePath, irFn("img-new"))
	if err == nil {
		t.Fatalf("expected the set to be refused while an unassessable holder exists")
	}
	for _, want := range []string{"delta.delta_thing.spare", "could not be assessed"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q: %v", want, err)
		}
	}
	if len(obs.destroyed) != 0 {
		t.Errorf("the refusal destroyed %v; it must destroy nothing", obs.destroyed)
	}
}

// The regression for the design error the real-provider tier found.
//
// Nivis evaluates with the recorded outputs injected, and an evaluation that
// can see a value BAKES IT IN: the reference leaf and the IR edge recording it
// are both gone from the graph the executor receives. Every other test here
// uses a stub that emits the same IR whatever the ledger holds, so they cannot
// see this, and the apply path happens to be safe because its first evaluation
// has an empty ledger. `nivis plan` seeds the ledger from state first, and
// reported a cascade as "no change".
//
// This stub bakes the way the real evaluator does. Falsified by setting Reeval
// to nil in PlanReport: the dependent is then reported as a no-op.
func bakingIR(bin, bottomImage string) func(*ledger.Ledger) []byte {
	return func(l *ledger.Ledger) []byte {
		midImage := `{"__ref":{"resource":"delta.delta_thing.bottom","path":["id"]}}`
		edges := `{"from":"delta.delta_thing.bottom","to":"delta.delta_thing.mid","via":"image"}`
		if outs, ok := l.Outputs["delta.delta_thing.bottom"]; ok {
			// The evaluation can see it, so it is a constant and the edge that
			// recorded the dependency is not emitted at all.
			midImage = fmt.Sprintf("%q", outs["id"])
			edges = ""
		}
		return []byte(fmt.Sprintf(`{
		  "schemaVersion":1,
		  "providers":{"delta":{"source":%q,"config":{}}},
		  "resources":[
		    {"id":"delta.delta_thing.bottom","provider":"delta","type":"delta_thing","name":"bottom",
		     "config":{"label":"b","image":%q}},
		    {"id":"delta.delta_thing.mid","provider":"delta","type":"delta_thing","name":"mid",
		     "config":{"label":"m","image":%s}}
		  ],
		  "dataSources":[],
		  "edges":[%s],
		  "nixConsumers":[]
		}`, bin, bottomImage, midImage, edges))
	}
}

func TestPlanReportsACascadeThroughABakedReference(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	bin := buildProvider(t, "provider-delta")
	statePath := t.TempDir() + "/state.json"

	if _, err := runCascade(t, statePath, bakingIR(bin, "img-old")); err != nil {
		t.Fatalf("initial apply: %v", err)
	}

	st, err := state.Open(statePath)
	if err != nil {
		t.Fatal(err)
	}
	mgr := plugin.NewManager()
	defer mgr.Close()
	d := &phase.Driver{
		Eval:    &phase.StubEvaluator{IRForLedger: bakingIR(bin, "img-new")},
		Manager: mgr, Store: st, Ledger: ledger.New(), MaxPhases: 10,
	}
	items, err := d.PlanReport(context.Background())
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	ops := map[string]plan.Op{}
	for _, it := range items {
		ops[it.ID] = it.Op
	}
	if ops["delta.delta_thing.bottom"] != plan.OpReplace {
		t.Fatalf("bottom reported as %v, want replace: the test is not exercising a cascade", ops["delta.delta_thing.bottom"])
	}
	if ops["delta.delta_thing.mid"] != plan.OpReplace {
		t.Errorf("mid reported as %v, want replace: the plan understated what the apply will destroy", ops["delta.delta_thing.mid"])
	}
}
