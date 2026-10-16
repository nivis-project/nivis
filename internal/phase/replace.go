// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nivis-project/nivis/internal/graph"
	"github.com/nivis-project/nivis/internal/ir"
	"github.com/nivis-project/nivis/internal/ledger"
	"github.com/nivis-project/nivis/internal/plan"
	"github.com/nivis-project/nivis/internal/progress"
	"github.com/nivis-project/nivis/internal/provider"
)

// replacePrologue computes the resources that must be replaced together, refuses
// if anything in state holds one of them without joining them, and otherwise
// destroys them in reverse dependency order so the forward loop recreates them.
//
// It runs at the top of every phase rather than once per run. A resource that
// only appears in the IR from a later phase can be in state from a previous run
// and need replacing, and a prologue that ran once could not see it. This
// introduces no assumption about configuration stability that the driver does
// not already make: a node is applied once and never reconsidered, so the
// working set here is "in state, not yet applied this run".
//
// Candidates are restricted to resources whose configuration is already fully
// resolved. An unknown produced by withholding an output and an unknown left by
// a reference this phase has not resolved are indistinguishable once encoded, so
// a resource waiting on a datasource or on a re-evaluated value would otherwise
// look like a dependent of something being replaced. A holder that cannot be
// assessed for that reason is refused below rather than guessed at, and the next
// phase, where it is resolved, reaches it.
func (d *Driver) replacePrologue(
	ctx context.Context, g *ir.Graph, applied map[string]bool,
) (map[string]bool, error) {
	stored, err := d.Store.List()
	if err != nil {
		// Failing to read state here is not "nothing depends on anything". This
		// guards a destructive step, so it fails closed, with the same wording
		// the per-node guard uses: the question was not answered.
		return nil, fmt.Errorf(
			"cannot decide what must be replaced: its dependents could not be determined, so destroying anything is not safe: %w", err)
	}
	if len(stored) == 0 {
		return nil, nil
	}
	inState := make(map[string]bool, len(stored))
	// References are resolved against STORED state, not against the run's
	// ledger. An apply builds its ledger as it goes, so at the top of a phase it
	// holds only what this run has already applied, and a resource that has been
	// in state since a previous run would look unresolvable. The question here
	// is about what already exists, which is exactly what state records.
	outputs := d.Ledger.ToGraphOutputs()
	for _, rs := range stored {
		inState[rs.ID] = true
		if _, already := outputs[rs.ID]; !already {
			outputs[rs.ID] = rs.Attrs
		}
	}
	res := graph.ResolveTFTF(g, outputs)

	candidates := map[string]bool{}
	for _, id := range res.FullyKnown {
		node := g.Nodes[id]
		if node == nil || node.Resource.IsData || applied[id] || !inState[id] {
			continue
		}
		candidates[id] = true
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	dag, err := graph.Build(g)
	if err != nil {
		return nil, fmt.Errorf("build the dependency graph: %w", err)
	}

	set, err := plan.ReplaceSet(ctx, plan.ReplaceSetInput{
		Graph:      g,
		Configs:    res.Configs,
		Candidates: candidates,
		Reeval:     d.reevalWithout(outputs),
		Plan: func(ctx context.Context, node *ir.ResourceNode, cfg map[string]interface{}) (plan.Op, error) {
			pr, _, _, err := d.planCandidate(ctx, g, node, cfg)
			return pr, err
		},
	})
	if err != nil {
		return nil, err
	}
	if len(set) <= 1 {
		// Nothing, or a single resource with no cascade. The ordinary per-node
		// path handles that case exactly as before, including its refusal.
		return nil, nil
	}

	member := make(map[string]bool, len(set))
	for _, id := range set {
		member[id] = true
	}
	if err := d.refuseUnjoinedHolders(g, dag, set, member, inState, candidates); err != nil {
		return nil, err
	}
	if err := d.destroyReplaceSet(ctx, g, res, dag, set, member); err != nil {
		return nil, err
	}
	return member, nil
}

// reevalWithout returns the re-evaluation seam the fixpoint needs: evaluate the
// configuration again with a set of resources' recorded outputs kept back.
//
// It cannot be done in process. Nivis evaluates with the recorded outputs
// injected, so an evaluation that can see a value bakes it in: the reference
// leaf AND the IR edge recording it are both absent from the graph the executor
// receives. Measured on the real postgres fixture, where seeding the ledger
// turned `{"__ref": ...}` into `"appdb"` and left `edges` empty. Dropping the
// target and evaluating again is the only way the dependency comes back.
//
// nil when there is no evaluator, which limits a set to the resources whose own
// configuration changed. Only unit tests that drive applyOne directly are in
// that position.
func (d *Driver) reevalWithout(outputs graph.Outputs) plan.Reevaluate {
	if d.Eval == nil {
		return nil
	}
	return func(ctx context.Context, withheld map[string]bool) (plan.Withheld, error) {
		l := ledger.New()
		l.Vars = d.Ledger.Vars
		l.Phase = d.Ledger.Phase
		kept := make(graph.Outputs, len(outputs))
		for id, attrs := range outputs {
			if withheld[id] {
				continue
			}
			kept[id] = attrs
			l.Append(id, attrs)
		}
		irJSON, err := d.Eval.Eval(ctx, l)
		if err != nil {
			return plan.Withheld{}, err
		}
		g, err := ir.IngestIR(irJSON)
		if err != nil {
			return plan.Withheld{}, err
		}
		dag, err := graph.Build(g)
		if err != nil {
			return plan.Withheld{}, err
		}
		return plan.Withheld{Graph: g, DAG: dag, Configs: graph.ResolveTFTF(g, kept).Configs}, nil
	}
}

// planCandidate plans one resource against its refreshed prior state, returning
// the operation. It also returns the client and schema so the destroy half does
// not fetch them again.
func (d *Driver) planCandidate(
	ctx context.Context, g *ir.Graph, node *ir.ResourceNode, cfg map[string]interface{},
) (plan.Op, provider.Client, provider.ResourceSchema, error) {
	prov, ok := g.Providers[node.Resource.Provider]
	if !ok {
		return plan.OpCreate, nil, provider.ResourceSchema{}, fmt.Errorf("provider %q not declared", node.Resource.Provider)
	}
	client, err := d.Manager.Client(node.Resource.Provider, prov.Source, prov.Config)
	if err != nil {
		return plan.OpCreate, nil, provider.ResourceSchema{}, fmt.Errorf("provider client: %w", err)
	}
	rs, err := plan.SchemaFor(ctx, client, node.Resource.Type)
	if err != nil {
		return plan.OpCreate, nil, provider.ResourceSchema{}, err
	}
	prior, exists, err := d.storedPrior(ctx, client, rs, node)
	if err != nil {
		return plan.OpCreate, nil, provider.ResourceSchema{}, err
	}
	if !exists {
		return plan.OpCreate, client, rs, nil
	}
	pr, err := plan.Plan(ctx, client, rs, node, cfg, prior)
	if err != nil {
		return plan.OpCreate, nil, provider.ResourceSchema{}, err
	}
	return pr.Op, client, rs, nil
}

// storedPrior reads a resource's prior state: upgraded through the provider when
// the schema has moved on, then refreshed.
//
// The answer is cached for the rest of the phase, because the prologue and the
// apply that follows it both need it and each asks the provider twice over (an
// UpgradeResourceState and a ReadResource). Without the cache, adding the
// prologue doubled every in-state resource's round trips. A phase is the right
// lifetime: it is one evaluation of the configuration, which is already the
// granularity at which the driver treats the world as fixed.
func (d *Driver) storedPrior(
	ctx context.Context, client provider.Client, rs provider.ResourceSchema, node *ir.ResourceNode,
) (map[string]interface{}, bool, error) {
	id := node.Resource.ID
	if hit, ok := d.priorCache[id]; ok {
		return hit.attrs, hit.exists, nil
	}
	stored, found, err := d.Store.Get(id)
	if err != nil {
		return nil, false, fmt.Errorf("read prior state for %q: %w", id, err)
	}
	if !found {
		return nil, false, nil
	}
	attrs, err := plan.UpgradeStored(ctx, client, rs, node, stored.SchemaVersion, stored.Attrs)
	if err != nil {
		return nil, false, err
	}
	prior, exists, err := d.priorState(ctx, client, rs, node, attrs)
	if err != nil {
		return nil, false, err
	}
	if d.priorCache != nil {
		d.priorCache[id] = priorEntry{attrs: prior, exists: exists}
	}
	return prior, exists, nil
}

// priorEntry is one cached prior state. `exists` is carried because "refreshed
// to nothing" (destroyed out of band) is a real answer, distinct from "not
// asked yet".
type priorEntry struct {
	attrs  map[string]interface{}
	exists bool
}

// forgetPrior drops a cached prior whose object has just been destroyed, so a
// later read in the same phase does not resurrect it.
func (d *Driver) forgetPrior(id string) { delete(d.priorCache, id) }

// refuseUnjoinedHolders refuses the whole set when anything in state holds a
// member without being replaced with it.
//
// Two different things land here and the message distinguishes them, because
// the remedies differ. A holder the provider plans as an in-place update keeps
// its reference and nothing in a cascade can keep that reference valid across
// the gap; that is what a create-before-destroy lifecycle is for. A holder that
// could not be assessed at all, because its own configuration waits on a value
// this phase has not resolved, is not a judgement about the provider: it is an
// admission that the question was not asked.
func (d *Driver) refuseUnjoinedHolders(
	g *ir.Graph, dag *graph.DAG, set []string, member, inState, candidates map[string]bool,
) error {
	unassessed := map[string]bool{}
	inPlace := map[string]bool{}
	for _, id := range set {
		for _, dep := range dag.Dependents(id) {
			if member[dep] || !inState[dep] {
				continue
			}
			if candidates[dep] {
				inPlace[dep] = true
			} else {
				unassessed[dep] = true
			}
		}
	}
	if len(inPlace) == 0 && len(unassessed) == 0 {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "cannot replace %s: destroying them requires destroying resources that hold them and are not replaced with them",
		strings.Join(set, ", "))
	if len(inPlace) > 0 {
		fmt.Fprintf(&b, ".\nThese hold one of them and the provider plans them as an in-place update, so a replacement cannot keep their reference valid: %s.\n"+
			"A create-before-destroy lifecycle is what this case needs; nivis does not have one yet. Remove or destroy them first, or change the configuration so nothing is replaced",
			strings.Join(sortedKeys(inPlace), ", "))
	}
	if len(unassessed) > 0 {
		fmt.Fprintf(&b, ".\nThese hold one of them and could not be assessed, because their own configuration waits on a value this phase has not resolved: %s",
			strings.Join(sortedKeys(unassessed), ", "))
	}
	return fmt.Errorf("%s", b.String())
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// destroyReplaceSet destroys the set's prior objects in reverse dependency
// order, clearing each state entry as its object goes.
//
// The order is what the whole prologue exists for: a backing service refuses to
// destroy an object another object holds, and answers in its own vocabulary,
// late, after other resources in the run have already changed.
//
// Each entry is cleared immediately after its object is destroyed rather than
// at the end, so an interrupted run never leaves state asserting that a
// destroyed object exists. That is bean nixform2-sw6c's rule applied to a set.
func (d *Driver) destroyReplaceSet(
	ctx context.Context, g *ir.Graph, res graph.ResolveResult, dag *graph.DAG, set []string, member map[string]bool,
) error {
	for _, id := range dag.DestroyOrder() {
		if !member[id] {
			continue
		}
		node := g.Nodes[id]
		if node.Resource.Meta != nil && node.Resource.Meta.Lifecycle != nil && node.Resource.Meta.Lifecycle.PreventDestroy {
			return fmt.Errorf("replacing %q requires destroying it, but lifecycle.preventDestroy is set", id)
		}
		_, client, rs, err := d.planCandidate(ctx, g, node, res.Configs[id])
		if err != nil {
			return err
		}
		prior, exists, err := d.storedPrior(ctx, client, rs, node)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		d.emit(progress.Event{
			Kind: progress.NodeStart, ID: id, Type: node.Resource.Type,
			Op: plan.OpReplace, Destroy: true,
		})
		started := time.Now()
		_, derr := client.Destroy(ctx, provider.DestroyRequest{
			Schema: rs, TypeName: node.Resource.Type, Stored: prior,
		})
		d.emit(progress.Event{
			Kind: progress.NodeDone, ID: id, Type: node.Resource.Type,
			Op: plan.OpReplace, Destroy: true, Duration: time.Since(started), Err: derr,
		})
		if derr != nil {
			return fmt.Errorf("replace %q: destroy prior: %w", id, derr)
		}
		if err := d.Store.Delete(id); err != nil {
			return fmt.Errorf("replace %q: destroyed the prior resource but could not clear its state entry: %w", id, err)
		}
		d.forgetPrior(id)
	}
	return nil
}
