// Copyright 2026 WeareTechnative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package plan runs the provider plan operation for a ready resource and renders
// a human-readable plan. It has no side effects. It depends on the
// version-neutral provider.Client, not a specific protocol.
package plan

import (
	"context"
	"fmt"
	"strings"

	"github.com/wearetechnative/nivis/internal/ir"
	"github.com/wearetechnative/nivis/internal/provider"
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

// Plan plans a ready resource with its resolved config (unresolved refs encoded
// as unknown by the backend) against its prior state (nil for a new resource).
// It returns the planned state for apply, the implied operation, and a
// human-readable summary.
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
	return Result{
		PlannedState:      pr.PlannedState,
		UnknownAfterApply: pr.UnknownAfterApply,
		Op:                op,
		Human:             renderPlan(node.Resource.ID, op, pr.UnknownAfterApply),
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
