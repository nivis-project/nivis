// Package apply runs the provider ApplyResourceChange RPC and persists the
// resulting computed outputs to the state store.
package apply

import (
	"context"
	"fmt"
	"strings"

	"github.com/wearetechnative/nixform/internal/ir"
	"github.com/wearetechnative/nixform/internal/plan"
	"github.com/wearetechnative/nixform/internal/state"
	"github.com/wearetechnative/nixform/internal/tfplugin6"
	"github.com/wearetechnative/nixform/internal/tfvalue"
)

// Apply calls ApplyResourceChange for a planned resource, extracts the now-known
// outputs, and writes them to the state store. State is persisted on success so
// a later failure in the same run leaves this resource's outputs recorded.
// It returns the resource's output attributes.
func Apply(
	ctx context.Context,
	client tfplugin6.ProviderClient,
	rs plan.ResourceSchema,
	node *ir.ResourceNode,
	resolvedCfg map[string]interface{},
	plannedState *tfplugin6.DynamicValue,
	store state.Store,
) (map[string]interface{}, error) {
	cfgDV, err := tfvalue.EncodeConfig(rs.ObjType, rs.Computed, resolvedCfg)
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	resp, err := client.ApplyResourceChange(ctx, &tfplugin6.ApplyResourceChange_Request{
		TypeName:     node.Resource.Type,
		Config:       cfgDV,
		PlannedState: plannedState,
		PriorState:   &tfplugin6.DynamicValue{Msgpack: nil},
	})
	if err != nil {
		return nil, fmt.Errorf("ApplyResourceChange: %w", err)
	}
	if err := diagErr(resp.GetDiagnostics()); err != nil {
		return nil, err
	}

	attrs, err := tfvalue.DecodeState(rs.ObjType, resp.GetNewState())
	if err != nil {
		return nil, fmt.Errorf("decode new state: %w", err)
	}

	if err := store.Set(state.ResourceState{
		ID:    node.Resource.ID,
		Type:  node.Resource.Type,
		Attrs: attrs,
	}); err != nil {
		return nil, fmt.Errorf("persist state for %q: %w", node.Resource.ID, err)
	}
	return attrs, nil
}

func diagErr(diags []*tfplugin6.Diagnostic) error {
	var msgs []string
	for _, d := range diags {
		if d.GetSeverity() == tfplugin6.Diagnostic_ERROR {
			msgs = append(msgs, strings.TrimSpace(d.GetSummary()+": "+d.GetDetail()))
		}
	}
	if len(msgs) > 0 {
		return fmt.Errorf("provider diagnostics: %s", strings.Join(msgs, "; "))
	}
	return nil
}
