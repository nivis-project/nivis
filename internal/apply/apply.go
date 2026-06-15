// Package apply runs the provider apply operation and persists the resulting
// computed outputs to the state store. Version-neutral (provider.Client).
package apply

import (
	"context"
	"fmt"

	"github.com/wearetechnative/nixform/internal/ir"
	"github.com/wearetechnative/nixform/internal/provider"
	"github.com/wearetechnative/nixform/internal/state"
)

// Apply applies a planned resource, extracts the now-known outputs, and writes
// them to the state store. State is persisted on success so a later failure in
// the same run leaves this resource's outputs recorded. Returns the outputs.
func Apply(
	ctx context.Context,
	client provider.Client,
	rs provider.ResourceSchema,
	node *ir.ResourceNode,
	resolvedCfg map[string]interface{},
	plannedState interface{},
	store state.Store,
) (map[string]interface{}, error) {
	res, err := client.Apply(ctx, provider.ApplyRequest{
		Schema:       rs,
		TypeName:     node.Resource.Type,
		ResolvedCfg:  resolvedCfg,
		PlannedState: plannedState,
	})
	if err != nil {
		return nil, err
	}
	if err := store.Set(state.ResourceState{
		ID:    node.Resource.ID,
		Type:  node.Resource.Type,
		Attrs: res.Attrs,
	}); err != nil {
		return nil, fmt.Errorf("persist state for %q: %w", node.Resource.ID, err)
	}
	return res.Attrs, nil
}
