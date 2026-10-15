// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package plan runs the provider plan operation for a ready resource and renders
// a human-readable plan. It has no side effects. It depends on the
// version-neutral provider.Client, not a specific protocol.
package plan

import (
	"context"
	"fmt"
	"strings"

	"github.com/nivis-project/nivis/internal/ir"
	"github.com/nivis-project/nivis/internal/provider"
)

// Op is the operation a plan implies for a resource.
type Op int

const (
	// OpCreate: the resource does not exist yet (no prior state).
	OpCreate Op = iota
	// OpUpdate: the resource exists and changes apply in place.
	OpUpdate
	// OpReplace: the resource exists but a force-new attribute changed, so it
	// must be destroyed and recreated.
	OpReplace
	// OpNoop: the resource exists and nothing changed — nothing to do.
	OpNoop
)

// Result is the outcome of planning one resource.
type Result struct {
	// PlannedState is an opaque backend handle carried into Apply.
	PlannedState interface{}
	// UnknownAfterApply lists attributes unknown in the planned state.
	UnknownAfterApply []string
	// Op is create / update / replace, decided from prior state + the provider's
	// RequiresReplace.
	Op    Op
	Human string
}

// SchemaFor fetches the schema for one resource type via the client.
func SchemaFor(ctx context.Context, client provider.Client, resourceType string) (provider.ResourceSchema, error) {
	return client.GetSchema(ctx, resourceType)
}

// UpgradeStored brings a stored resource's attributes up to the schema the
// provider declares now, and is the ONE place that compares the two versions.
//
// It lives here, beside SchemaFor, because both the phase driver and the refresh
// path read stored state and both need this. It was a method on the driver
// first, and the refresh path then went two hours without it (bean
// nixform2-ff03); a second copy would have made the next divergence quieter
// rather than louder.
//
// An ABSENT recorded version (zero) means the state is already current, not
// version zero: a document written before the field existed carries none, and
// asking a provider to upgrade 0 -> N on current state invites it to do
// anything at all.
//
// A version NEWER than the provider's is refused rather than repaired. A
// provider cannot downgrade its own state, so continuing would encode newer
// attributes against an older schema.
func UpgradeStored(
	ctx context.Context, client provider.Client, rs provider.ResourceSchema,
	node *ir.ResourceNode, storedVersion int64, storedAttrs map[string]interface{},
) (map[string]interface{}, error) {
	if storedVersion == 0 || storedVersion == rs.Version {
		return storedAttrs, nil
	}
	if storedVersion > rs.Version {
		return nil, fmt.Errorf(
			"state for %q was written at schema version %d by a newer %s provider than the one configured, which declares version %d.\n"+
				"A provider cannot downgrade its own state. Upgrade the provider to work with this state",
			node.Resource.ID, storedVersion, node.Resource.Provider, rs.Version)
	}
	res, err := client.UpgradeState(ctx, provider.UpgradeStateRequest{
		Schema:      rs,
		TypeName:    node.Resource.Type,
		FromVersion: storedVersion,
		Stored:      storedAttrs,
	})
	if err != nil {
		return nil, fmt.Errorf("upgrade state for %q from schema version %d to %d: %w",
			node.Resource.ID, storedVersion, rs.Version, err)
	}
	return res.Attrs, nil
}

// Plan plans a ready resource with its resolved config (unresolved refs encoded
// as unknown by the backend) against its prior state (nil for a new resource).
// It returns the planned state for apply, the implied operation, and a
// human-readable summary.
//
// A REPLACEMENT is planned twice. The operation can only be decided against the
// real prior state, because that is what the provider compares to report
// requires-replace. But the object holding those prior values is about to be
// destroyed, so planning its replacement against it inherits computed values the
// config never set and that will not exist afterwards. The second plan uses a
// null prior, which is what Terraform does and what a create would see.
func Plan(ctx context.Context, client provider.Client, rs provider.ResourceSchema, node *ir.ResourceNode, resolvedCfg map[string]interface{}, prior map[string]interface{}) (Result, error) {
	pr, err := client.Plan(ctx, provider.PlanRequest{
		Schema:      rs,
		TypeName:    node.Resource.Type,
		ResolvedCfg: resolvedCfg,
		Prior:       prior,
	})
	if err != nil {
		return Result{}, err
	}
	op := OpCreate
	switch {
	case prior == nil:
		op = OpCreate
	case pr.RequiresReplace:
		op = OpReplace
	case pr.NoOp:
		op = OpNoop
	default:
		op = OpUpdate
	}

	planned, unknown := pr.PlannedState, pr.UnknownAfterApply
	if op == OpReplace {
		cr, err := client.Plan(ctx, provider.PlanRequest{
			Schema:      rs,
			TypeName:    node.Resource.Type,
			ResolvedCfg: resolvedCfg,
			Prior:       nil,
		})
		if err != nil {
			return Result{}, fmt.Errorf("plan the create half of replacing %q: %w", node.Resource.ID, err)
		}
		planned, unknown = cr.PlannedState, cr.UnknownAfterApply
	}

	return Result{
		PlannedState:      planned,
		UnknownAfterApply: unknown,
		Op:                op,
		Human:             renderPlan(node.Resource.ID, op, unknown),
	}, nil
}

func renderPlan(id string, op Op, unknown []string) string {
	var b strings.Builder
	switch op {
	case OpUpdate:
		fmt.Fprintf(&b, "%s will be updated in place", id)
	case OpReplace:
		fmt.Fprintf(&b, "%s will be replaced (destroy and re-create)", id)
	case OpNoop:
		fmt.Fprintf(&b, "%s is up to date (no change)", id)
	default:
		fmt.Fprintf(&b, "%s will be created", id)
	}
	if len(unknown) > 0 {
		fmt.Fprintf(&b, " (%s known after apply)", strings.Join(unknown, ", "))
	}
	return b.String()
}
