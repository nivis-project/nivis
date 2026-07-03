// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package phase drives resolution to a fixpoint across N phases (DESIGN D3, the
// thesis). Each phase: re-evaluate Nix with the accumulated outputs ledger,
// ingest the resulting IR, apply the resources that are now fully known, and
// append their computed outputs. Repeat until no phase resolves a new value.
//
// The phase count is driven by Nix-mediated (__derived) dependencies: a derived
// value only becomes concrete after its inputs are in the ledger AND Nix is
// re-evaluated, so each such hop needs its own phase. That is why a single apply
// is insufficient and the loop is required.
package phase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/nivis-project/nivis/internal/apply"
	"github.com/nivis-project/nivis/internal/graph"
	"github.com/nivis-project/nivis/internal/ir"
	"github.com/nivis-project/nivis/internal/ledger"
	"github.com/nivis-project/nivis/internal/plan"
	"github.com/nivis-project/nivis/internal/provider"
	"github.com/nivis-project/nivis/internal/state"
)

// ProviderManager spawns/pools provider clients by identity. internal/plugin's
// Manager satisfies this; the interface keeps phase testable.
type ProviderManager interface {
	Client(identity, path string, config map[string]interface{}) (provider.Client, error)
}

// Driver runs the phased-eval loop.
type Driver struct {
	Eval    NixEvaluator
	Manager ProviderManager
	Store   state.Store
	Ledger  *ledger.Ledger
	// LedgerPath, if set, persists the ledger after each phase (0600).
	LedgerPath string
	// MaxPhases caps the loop as a safety net (0 = a generous default).
	MaxPhases int
	// NoRefresh disables reading a resource's real state from the provider before
	// planning (the `--refresh=false` opt-out). When false (the default), plan/
	// apply refresh prior state so drift and out-of-band deletion are seen.
	NoRefresh bool
	// NoBuild disables realising __build leaves (the `--no-build` opt-out): the
	// store paths are used as-is, assumed already built. Default (false) realises.
	NoBuild bool
	// Realiser builds a Nix store path (a derivation output) referenced by a
	// __build leaf. nil uses the default (nix-store --realise). The seam lets
	// tests inject a stub.
	Realiser Realiser
}

// Realiser builds Nix store paths referenced by __build leaves in resource
// config. Realise is given the absolute store path of a build output; it must
// ensure that path is valid (built) in the store, building the derivation if
// necessary. internal/plugin's nix realiser satisfies this.
type Realiser interface {
	Realise(ctx context.Context, storePath string) error
}

// priorState returns the prior state for an in-state resource. By default it
// refreshes — reads the resource through the provider so drift is seen and an
// out-of-band deletion surfaces as an empty read (which callers treat as a
// create). With NoRefresh it returns the stored attributes unchanged. The bool
// reports whether the resource still exists (false => no prior / deleted).
func (d *Driver) priorState(ctx context.Context, client provider.Client, rs provider.ResourceSchema, node *ir.ResourceNode, stored map[string]interface{}) (map[string]interface{}, bool, error) {
	if d.NoRefresh {
		return stored, len(stored) > 0, nil
	}
	rr, err := client.Read(ctx, provider.ReadRequest{
		Schema:   rs,
		TypeName: node.Resource.Type,
		Stored:   stored,
	})
	if err != nil {
		return nil, false, fmt.Errorf("refresh %q: %w", node.Resource.ID, err)
	}
	if len(rr.Attrs) == 0 {
		// Read returned empty: the resource was deleted out-of-band. No prior.
		return nil, false, nil
	}
	return rr.Attrs, true, nil
}

// Result summarizes a completed run.
// AppliedNode is one node that resolved in a phase, with its kind so a renderer
// can distinguish a datasource READ from a resource apply, and the operation a
// resource resolved as so the renderer reports the real change type (not always a
// create). Op is meaningful only when IsData is false.
type AppliedNode struct {
	ID     string
	IsData bool    // true => the node was READ (a datasource), not applied
	Op     plan.Op // the resource's resolved op (create/update/replace/no-op); ignored for datasources
}

type Result struct {
	AppliedPhases int               // number of phases that applied >=1 resource
	Applied       []string          // resource ids applied, in order
	Phases        [][]AppliedNode   // ids resolved in each phase, in phase order (reporting only)
	FinalLedger   *ledger.Ledger    // the accumulated outputs
	LastIR        *ir.Graph         // the final phase's ingested IR (for consumer checks)
	Outputs       map[string]string // flattened "<id>.<attr>" -> string, for asserts
}

// Run executes the loop to a fixpoint. It returns an error if, at fixpoint,
// resources remain unapplied (naming them and the inputs they await).
func (d *Driver) Run(ctx context.Context) (*Result, error) {
	if d.Ledger == nil {
		d.Ledger = ledger.New()
	}
	maxPhases := d.MaxPhases
	if maxPhases <= 0 {
		maxPhases = 50
	}

	applied := map[string]bool{}
	var appliedOrder []string
	var phases [][]AppliedNode
	appliedPhases := 0
	var lastGraph *ir.Graph

	for phaseNum := 0; phaseNum < maxPhases; phaseNum++ {
		d.Ledger.Phase = phaseNum

		irJSON, err := d.Eval.Eval(ctx, d.Ledger)
		if err != nil {
			return nil, fmt.Errorf("phase %d: eval: %w", phaseNum, err)
		}
		g, err := ir.IngestIR(irJSON)
		if err != nil {
			return nil, fmt.Errorf("phase %d: ingest: %w", phaseNum, err)
		}
		lastGraph = g

		// Resolve TF->TF refs against the ledger; FullyKnown = no unresolved ref
		// AND no derived leaf remaining in this IR.
		res := graph.ResolveTFTF(g, d.Ledger.ToGraphOutputs())

		progressed := false
		var thisPhase []AppliedNode
		for _, id := range res.FullyKnown {
			if applied[id] {
				continue
			}
			node := g.Nodes[id]
			// A datasource is READ (never planned/applied/stored); a resource is
			// applied. Both feed their outputs into the ledger so dependents
			// resolve. They share this readiness loop, so a datasource whose
			// config depends on a resource output reads in a later phase.
			var outs map[string]interface{}
			var op plan.Op
			var err error
			if node.Resource.IsData {
				outs, err = d.readOne(ctx, g, node, res.Configs[id])
				if err != nil {
					return nil, fmt.Errorf("phase %d: read datasource %q: %w", phaseNum, id, err)
				}
			} else {
				outs, op, err = d.applyOne(ctx, g, node, res.Configs[id])
				if err != nil {
					return nil, fmt.Errorf("phase %d: apply %q: %w", phaseNum, id, err)
				}
			}
			d.Ledger.Append(id, outs)
			applied[id] = true
			appliedOrder = append(appliedOrder, id)
			thisPhase = append(thisPhase, AppliedNode{ID: id, IsData: node.Resource.IsData, Op: op})
			progressed = true
		}

		if progressed {
			phases = append(phases, thisPhase)
			appliedPhases++
			if d.LedgerPath != "" {
				if err := d.Ledger.Save(d.LedgerPath); err != nil {
					return nil, fmt.Errorf("phase %d: save ledger: %w", phaseNum, err)
				}
			}
		}

		// Done when every resource in the current IR has been applied.
		if allApplied(g, applied) {
			return d.finish(appliedPhases, appliedOrder, phases, lastGraph), nil
		}

		// Fixpoint: a phase that resolved nothing new and work remains is stuck.
		if !progressed {
			return nil, stuckError(g, applied, d.Ledger)
		}
	}
	return nil, fmt.Errorf("phase loop exceeded %d phases without reaching fixpoint", maxPhases)
}

// PlanItem is one resource's planned operation, for `nivis plan`.
type PlanItem struct {
	ID   string
	Type string
	Op   plan.Op
}

// PlanReport evaluates the configuration once with the stored outputs injected
// and reports each resource's operation (create / update / replace / no-op)
// without applying. Resources already in state are planned against their prior
// state via the provider (so an unchanged resource reports no-op); resources not
// in state are reported as creates. It is side-effect free.
func (d *Driver) PlanReport(ctx context.Context) ([]PlanItem, error) {
	if d.Ledger == nil {
		d.Ledger = ledger.New()
	}
	// Seed the ledger from stored state so refs to already-applied resources
	// resolve (a re-plan of an applied graph is then fully known).
	if stored, err := d.Store.List(); err == nil {
		for _, rs := range stored {
			d.Ledger.Append(rs.ID, rs.Attrs)
		}
	}
	irJSON, err := d.Eval.Eval(ctx, d.Ledger)
	if err != nil {
		return nil, err
	}
	g, err := ir.IngestIR(irJSON)
	if err != nil {
		return nil, err
	}

	// Read the (side-effect-free) datasources into the ledger before classifying
	// resources, so a resource whose config reads a datasource is FullyKnown and
	// is planned against its provider (reporting its true op) rather than falling
	// into the "in state but not resolvable -> update pending" fallback. Datasources
	// are never planned/applied/stored; this read mirrors the apply loop. We iterate
	// to a fixpoint so a datasource that depends on a stored resource (already
	// seeded) or on another datasource still reads. Read-only; safe to repeat.
	readData := map[string]bool{}
	for {
		res := graph.ResolveTFTF(g, d.Ledger.ToGraphOutputs())
		known := map[string]bool{}
		for _, id := range res.FullyKnown {
			known[id] = true
		}
		progressed := false
		for _, id := range g.Order {
			node := g.Nodes[id]
			if !node.Resource.IsData || readData[id] || !known[id] {
				continue
			}
			outs, rerr := d.readOne(ctx, g, node, res.Configs[id])
			if rerr != nil {
				return nil, fmt.Errorf("plan: read datasource %q: %w", id, rerr)
			}
			d.Ledger.Append(id, outs)
			readData[id] = true
			progressed = true
		}
		if !progressed {
			break
		}
	}

	res := graph.ResolveTFTF(g, d.Ledger.ToGraphOutputs())
	resolvable := map[string]bool{}
	for _, id := range res.FullyKnown {
		resolvable[id] = true
	}

	var items []PlanItem
	for _, id := range g.Order {
		node := g.Nodes[id]
		// Datasources are read, not created; they never appear in the plan.
		if node.Resource.IsData {
			continue
		}
		item := PlanItem{ID: id, Type: node.Resource.Type, Op: plan.OpCreate}

		stored, found, err := d.Store.Get(id)
		if err != nil {
			return nil, err
		}
		// Only plan against the provider when the resource is in state AND its
		// config is fully resolvable now; otherwise it's a create (or an
		// as-yet-unresolvable future phase, which we report as a create).
		if found && resolvable[id] {
			prov, ok := g.Providers[node.Resource.Provider]
			if !ok {
				return nil, fmt.Errorf("provider %q not declared", node.Resource.Provider)
			}
			client, err := d.Manager.Client(node.Resource.Provider, prov.Source, prov.Config)
			if err != nil {
				return nil, fmt.Errorf("provider client: %w", err)
			}
			rs, err := plan.SchemaFor(ctx, client, node.Resource.Type)
			if err != nil {
				return nil, err
			}
			// Refresh prior state (unless --refresh=false): a drifted resource is
			// planned against its real state; one deleted out-of-band reads empty
			// and is reported as a create.
			prior, exists, err := d.priorState(ctx, client, rs, node, stored.Attrs)
			if err != nil {
				return nil, err
			}
			if !exists {
				item.Op = plan.OpCreate
				items = append(items, item)
				continue
			}
			pr, err := plan.Plan(ctx, client, rs, node, res.Configs[id], prior)
			if err != nil {
				return nil, err
			}
			item.Op = pr.Op
		} else if found {
			// In state but not resolvable this pass — treat as an update pending.
			item.Op = plan.OpUpdate
		}
		items = append(items, item)
	}
	return items, nil
}

// outputPrefix marks a nixConsumer as a declared stack output: id "output.<name>".
const outputPrefix = "output."

// ResolveOutputs resolves the run's declared stack outputs to concrete values. It
// seeds the ledger from current state and re-evaluates read-only (like
// PlanReport), so a fully-applied stack's outputs come back concrete from the Nix
// eval (a consumer's value is resolved by the eval itself, with the ledger
// injected). It returns name->value for every `output.<name>` consumer, unwrapped
// from its { value } shape.
func (d *Driver) ResolveOutputs(ctx context.Context) (map[string]interface{}, error) {
	if d.Ledger == nil {
		d.Ledger = ledger.New()
	}
	if stored, err := d.Store.List(); err == nil {
		for _, rs := range stored {
			d.Ledger.Append(rs.ID, rs.Attrs)
		}
	}
	irJSON, err := d.Eval.Eval(ctx, d.Ledger)
	if err != nil {
		return nil, err
	}
	g, err := ir.IngestIR(irJSON)
	if err != nil {
		return nil, err
	}

	// Datasources are not persisted to state, so an output (or consumer) that
	// references a datasource result is still an unresolved ref after the
	// state-seeded eval. Re-read the ready datasources (reads are pure/idempotent),
	// add their outputs to the ledger, and re-eval so those refs resolve. One pass
	// suffices for datasources whose config is known from state; a datasource that
	// itself depends on an unread datasource would need more, but the common case
	// (datasource read directly, or from a resource in state) is one pass.
	res := graph.ResolveTFTF(g, d.Ledger.ToGraphOutputs())
	readAny := false
	for _, id := range res.FullyKnown {
		node := g.Nodes[id]
		if !node.Resource.IsData {
			continue
		}
		if _, already := d.Ledger.Outputs[id]; already {
			continue
		}
		outs, err := d.readOne(ctx, g, node, res.Configs[id])
		if err != nil {
			return nil, fmt.Errorf("read datasource %q for outputs: %w", id, err)
		}
		d.Ledger.Append(id, outs)
		readAny = true
	}
	if readAny {
		irJSON, err = d.Eval.Eval(ctx, d.Ledger)
		if err != nil {
			return nil, err
		}
		g, err = ir.IngestIR(irJSON)
		if err != nil {
			return nil, err
		}
	}

	out := map[string]interface{}{}
	for _, c := range g.Consumers {
		name, ok := strings.CutPrefix(c.ID, outputPrefix)
		if !ok {
			continue
		}
		// the declared output's value is wrapped as { value = <expr> }.
		if v, has := c.Value["value"]; has {
			out[name] = v
		} else {
			out[name] = c.Value
		}
	}
	return out, nil
}

// readOne reads a datasource: it fetches the datasource schema, calls the
// provider's ReadDataSource with the (fully-known) resolved config, and returns
// the read attributes. A datasource is never planned, applied, written to state,
// or destroyed; its outputs feed the ledger like a resource's so dependents
// resolve against them.
func (d *Driver) readOne(ctx context.Context, g *ir.Graph, node *ir.ResourceNode, resolvedCfg map[string]interface{}) (map[string]interface{}, error) {
	prov, ok := g.Providers[node.Resource.Provider]
	if !ok {
		return nil, fmt.Errorf("provider %q not declared", node.Resource.Provider)
	}
	client, err := d.Manager.Client(node.Resource.Provider, prov.Source, prov.Config)
	if err != nil {
		return nil, fmt.Errorf("provider client: %w", err)
	}
	rs, err := client.GetDataSourceSchema(ctx, node.Resource.Type)
	if err != nil {
		return nil, err
	}
	res, err := client.ReadDataSource(ctx, provider.ReadDataSourceRequest{
		Schema:      rs,
		TypeName:    node.Resource.Type,
		ResolvedCfg: resolvedCfg,
	})
	if err != nil {
		return nil, err
	}
	return res.Attrs, nil
}

// applyOne fetches the resource's schema, reads any prior state, plans against
// it (encoding unresolved refs as unknown), and applies the implied operation:
// create (no prior), update in place, or replace (destroy the prior resource
// then create). Returns the computed outputs.
func (d *Driver) applyOne(ctx context.Context, g *ir.Graph, node *ir.ResourceNode, resolvedCfg map[string]interface{}) (map[string]interface{}, plan.Op, error) {
	prov, ok := g.Providers[node.Resource.Provider]
	if !ok {
		return nil, plan.OpCreate, fmt.Errorf("provider %q not declared", node.Resource.Provider)
	}
	client, err := d.Manager.Client(node.Resource.Provider, prov.Source, prov.Config)
	if err != nil {
		return nil, plan.OpCreate, fmt.Errorf("provider client: %w", err)
	}
	rs, err := plan.SchemaFor(ctx, client, node.Resource.Type)
	if err != nil {
		return nil, plan.OpCreate, err
	}

	// Realise any __build leaves in this resource's config (Nix build outputs the
	// provider must read) and substitute the concrete path. Done per resource, as
	// it becomes ready, so only builds reachable this phase run — and a build that
	// depends on an earlier resource's output is realised in the later phase once
	// the config re-evaluates. `nivis` evaluates (not builds), so without this the
	// provider would see an unbuilt store path. `--no-build` skips it.
	if err := d.realiseBuilds(ctx, node.Resource.ID, resolvedCfg); err != nil {
		return nil, plan.OpCreate, err
	}

	// Prior state for this resource id, if it was applied in an earlier run.
	// By default this is refreshed from the provider (so drift is seen and an
	// out-of-band deletion becomes a create); --refresh=false uses stored state.
	var prior map[string]interface{}
	if stored, found, err := d.Store.Get(node.Resource.ID); err != nil {
		return nil, plan.OpCreate, fmt.Errorf("read prior state for %q: %w", node.Resource.ID, err)
	} else if found {
		prior, _, err = d.priorState(ctx, client, rs, node, stored.Attrs)
		if err != nil {
			return nil, plan.OpCreate, err
		}
	}

	pr, err := plan.Plan(ctx, client, rs, node, resolvedCfg, prior)
	if err != nil {
		return nil, plan.OpCreate, err
	}

	// The op the driver reports for this node (reporting only; behaviour below is
	// unchanged). It is exactly what plan computed.
	switch pr.Op {
	case plan.OpNoop:
		// Nothing changed: don't touch the provider. Keep the stored state and
		// surface the prior attributes as this resource's outputs so dependents
		// still resolve.
		return prior, plan.OpNoop, nil
	case plan.OpReplace:
		// Refuse if the prior resource is protected; otherwise destroy it first so
		// nothing is orphaned, then create the new one (prior=nil => create).
		if node.Resource.Meta != nil && node.Resource.Meta.Lifecycle != nil && node.Resource.Meta.Lifecycle.PreventDestroy {
			return nil, plan.OpReplace, fmt.Errorf("replace of %q requires destroying it, but lifecycle.preventDestroy is set", node.Resource.ID)
		}
		if _, err := client.Destroy(ctx, provider.DestroyRequest{
			Schema:   rs,
			TypeName: node.Resource.Type,
			Stored:   prior,
		}); err != nil {
			return nil, plan.OpReplace, fmt.Errorf("replace %q: destroy prior: %w", node.Resource.ID, err)
		}
		outs, err := apply.Apply(ctx, client, rs, node, resolvedCfg, pr.PlannedState, nil, d.Store)
		return outs, plan.OpReplace, err
	default:
		// OpCreate (prior nil) or OpUpdate (prior carried into apply for in-place).
		outs, err := apply.Apply(ctx, client, rs, node, resolvedCfg, pr.PlannedState, prior, d.Store)
		return outs, pr.Op, err
	}
}

// realiseBuilds walks a resolved config tree, replacing every __build leaf
// ({"__build":{"path":...}}) with its concrete store path after ensuring the path
// is built (via d.Realiser, unless NoBuild). It mutates maps/slices in place.
func (d *Driver) realiseBuilds(ctx context.Context, owner string, cfg map[string]interface{}) error {
	for k, v := range cfg {
		nv, err := d.realiseValue(ctx, owner, v)
		if err != nil {
			return err
		}
		cfg[k] = nv
	}
	return nil
}

func (d *Driver) realiseValue(ctx context.Context, owner string, v interface{}) (interface{}, error) {
	switch t := v.(type) {
	case map[string]interface{}:
		if path, ok := buildPath(t); ok {
			if !d.NoBuild {
				r := d.Realiser
				if r == nil {
					r = nixRealiser{}
				}
				if err := r.Realise(ctx, storeRoot(path)); err != nil {
					return nil, fmt.Errorf("realise build for %q (%s): %w", owner, path, err)
				}
			}
			return path, nil // substitute the __build leaf with its path string
		}
		for k, child := range t {
			nv, err := d.realiseValue(ctx, owner, child)
			if err != nil {
				return nil, err
			}
			t[k] = nv
		}
		return t, nil
	case []interface{}:
		for i, child := range t {
			nv, err := d.realiseValue(ctx, owner, child)
			if err != nil {
				return nil, err
			}
			t[i] = nv
		}
		return t, nil
	default:
		return v, nil
	}
}

// buildPath returns the path of a {"__build":{"path":...}} leaf, or false.
func buildPath(m map[string]interface{}) (string, bool) {
	b, ok := m["__build"].(map[string]interface{})
	if !ok {
		return "", false
	}
	p, ok := b["path"].(string)
	return p, ok && p != ""
}

// storeRoot reduces /nix/store/<hash>-<name>/sub/file to the store root
// /nix/store/<hash>-<name> (the realisable output path). A path that is not under
// /nix/store is returned unchanged (realise will surface a clear error).
func storeRoot(path string) string {
	const prefix = "/nix/store/"
	if !strings.HasPrefix(path, prefix) {
		return path
	}
	rest := path[len(prefix):]
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		return prefix + rest[:i]
	}
	return path
}

func (d *Driver) finish(phaseCount int, order []string, groups [][]AppliedNode, last *ir.Graph) *Result {
	flat := map[string]string{}
	for id, attrs := range d.Ledger.Outputs {
		for k, v := range attrs {
			if s, ok := v.(string); ok {
				flat[id+"."+k] = s
			}
		}
	}
	return &Result{
		AppliedPhases: phaseCount,
		Applied:       order,
		Phases:        groups,
		FinalLedger:   d.Ledger,
		LastIR:        last,
		Outputs:       flat,
	}
}

func allApplied(g *ir.Graph, applied map[string]bool) bool {
	for _, id := range g.Order {
		if !applied[id] {
			return false
		}
	}
	return true
}

// stuckError builds an actionable error naming each unapplied resource and the
// inputs it still awaits (derived inputs not in the ledger, or ref targets not
// yet applied).
func stuckError(g *ir.Graph, applied map[string]bool, l *ledger.Ledger) error {
	var lines []string
	for _, id := range g.Order {
		if applied[id] {
			continue
		}
		var awaiting []string
		for _, ref := range g.Nodes[id].Refs {
			switch ref.Class {
			case ir.ClassStarToNix:
				for _, in := range ref.Inputs {
					if !knownKey(l, in) {
						awaiting = append(awaiting, in)
					}
				}
			case ir.ClassTFTF:
				if !l.Has(ref.Target) {
					awaiting = append(awaiting, ref.Target)
				}
			}
		}
		sort.Strings(awaiting)
		lines = append(lines, fmt.Sprintf("%s awaits [%s]", id, strings.Join(dedup(awaiting), ", ")))
	}
	sort.Strings(lines)
	return fmt.Errorf("unresolvable at fixpoint (cycle or missing producer): %s", strings.Join(lines, "; "))
}

// knownKey checks a "<id>.<attr>" input against the ledger (id may contain dots).
func knownKey(l *ledger.Ledger, key string) bool {
	idx := strings.LastIndex(key, ".")
	if idx < 0 {
		return false
	}
	return l.Known(key[:idx], key[idx+1:])
}

func dedup(ss []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
