// Copyright 2026 WeareTechnative B.V. and the terrae-nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package plan runs the provider plan operation for a ready resource and renders
// a human-readable plan. It has no side effects. It depends on the
// version-neutral provider.Client, not a specific protocol.
package plan

import (
	"context"
	"fmt"
	"strings"

	"github.com/wearetechnative/terrae-nivis/internal/ir"
	"github.com/wearetechnative/terrae-nivis/internal/provider"
)

// Result is the outcome of planning one resource.
type Result struct {
	// PlannedState is an opaque backend handle carried into Apply.
	PlannedState interface{}
	// UnknownAfterApply lists attributes unknown in the planned state.
	UnknownAfterApply []string
	Human             string
}

// SchemaFor fetches the schema for one resource type via the client.
func SchemaFor(ctx context.Context, client provider.Client, resourceType string) (provider.ResourceSchema, error) {
	return client.GetSchema(ctx, resourceType)
}

// Plan plans a ready resource with its resolved config (unresolved refs encoded
// as unknown by the backend). It returns the planned state for apply plus a
// human-readable summary.
func Plan(ctx context.Context, client provider.Client, rs provider.ResourceSchema, node *ir.ResourceNode, resolvedCfg map[string]interface{}) (Result, error) {
	pr, err := client.Plan(ctx, provider.PlanRequest{
		Schema:      rs,
		TypeName:    node.Resource.Type,
		ResolvedCfg: resolvedCfg,
	})
	if err != nil {
		return Result{}, err
	}
	return Result{
		PlannedState:      pr.PlannedState,
		UnknownAfterApply: pr.UnknownAfterApply,
		Human:             renderPlan(node.Resource.ID, pr.UnknownAfterApply),
	}, nil
}

func renderPlan(id string, unknown []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s will be created", id)
	if len(unknown) > 0 {
		fmt.Fprintf(&b, " (%s known after apply)", strings.Join(unknown, ", "))
	}
	return b.String()
}
