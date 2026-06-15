// Copyright 2026 WeareTechnative B.V. and the terrae-nivis authors
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

	"github.com/wearetechnative/terrae-nivis/internal/apply"
	"github.com/wearetechnative/terrae-nivis/internal/graph"
	"github.com/wearetechnative/terrae-nivis/internal/ir"
	"github.com/wearetechnative/terrae-nivis/internal/ledger"
	"github.com/wearetechnative/terrae-nivis/internal/plan"
	"github.com/wearetechnative/terrae-nivis/internal/provider"
	"github.com/wearetechnative/terrae-nivis/internal/state"
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
}

// Result summarizes a completed run.
type Result struct {
	AppliedPhases int               // number of phases that applied >=1 resource
	Applied       []string          // resource ids applied, in order
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
		for _, id := range res.FullyKnown {
			if applied[id] {
				continue
			}
			node := g.Nodes[id]
			outs, err := d.applyOne(ctx, g, node, res.Configs[id])
			if err != nil {
				return nil, fmt.Errorf("phase %d: apply %q: %w", phaseNum, id, err)
			}
			d.Ledger.Append(id, outs)
			applied[id] = true
			appliedOrder = append(appliedOrder, id)
			progressed = true
		}

		if progressed {
			appliedPhases++
			if d.LedgerPath != "" {
				if err := d.Ledger.Save(d.LedgerPath); err != nil {
					return nil, fmt.Errorf("phase %d: save ledger: %w", phaseNum, err)
				}
			}
		}

		// Done when every resource in the current IR has been applied.
		if allApplied(g, applied) {
			return d.finish(appliedPhases, appliedOrder, lastGraph), nil
		}

		// Fixpoint: a phase that resolved nothing new and work remains is stuck.
		if !progressed {
			return nil, stuckError(g, applied, d.Ledger)
		}
	}
	return nil, fmt.Errorf("phase loop exceeded %d phases without reaching fixpoint", maxPhases)
}

// applyOne fetches the resource's schema, reads any prior state, plans against
// it (encoding unresolved refs as unknown), and applies the implied operation:
// create (no prior), update in place, or replace (destroy the prior resource
// then create). Returns the computed outputs.
func (d *Driver) applyOne(ctx context.Context, g *ir.Graph, node *ir.ResourceNode, resolvedCfg map[string]interface{}) (map[string]interface{}, error) {
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

	// Prior state for this resource id, if it was applied in an earlier run.
	var prior map[string]interface{}
	if stored, found, err := d.Store.Get(node.Resource.ID); err != nil {
		return nil, fmt.Errorf("read prior state for %q: %w", node.Resource.ID, err)
	} else if found {
		prior = stored.Attrs
	}

	pr, err := plan.Plan(ctx, client, rs, node, resolvedCfg, prior)
	if err != nil {
		return nil, err
	}

	switch pr.Op {
	case plan.OpReplace:
		// Refuse if the prior resource is protected; otherwise destroy it first so
		// nothing is orphaned, then create the new one (prior=nil => create).
		if node.Resource.Meta != nil && node.Resource.Meta.Lifecycle != nil && node.Resource.Meta.Lifecycle.PreventDestroy {
			return nil, fmt.Errorf("replace of %q requires destroying it, but lifecycle.preventDestroy is set", node.Resource.ID)
		}
		if _, err := client.Destroy(ctx, provider.DestroyRequest{
			Schema:   rs,
			TypeName: node.Resource.Type,
			Stored:   prior,
		}); err != nil {
			return nil, fmt.Errorf("replace %q: destroy prior: %w", node.Resource.ID, err)
		}
		return apply.Apply(ctx, client, rs, node, resolvedCfg, pr.PlannedState, nil, d.Store)
	default:
		// OpCreate (prior nil) or OpUpdate (prior carried into apply for in-place).
		return apply.Apply(ctx, client, rs, node, resolvedCfg, pr.PlannedState, prior, d.Store)
	}
}

func (d *Driver) finish(phases int, order []string, last *ir.Graph) *Result {
	flat := map[string]string{}
	for id, attrs := range d.Ledger.Outputs {
		for k, v := range attrs {
			if s, ok := v.(string); ok {
				flat[id+"."+k] = s
			}
		}
	}
	return &Result{
		AppliedPhases: phases,
		Applied:       order,
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
