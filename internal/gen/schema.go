package gen

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/wearetechnative/nixform/internal/tfplugin6"
)

// Manager is the provider-client seam (internal/plugin.Manager satisfies it).
type Manager interface {
	Client(identity, path string) (tfplugin6.ProviderClient, error)
}

// Fetch spawns the provider at path under identity, calls GetProviderSchema, and
// returns the normalized resource models (sorted by type for determinism).
func Fetch(ctx context.Context, mgr Manager, identity, path string) ([]Resource, error) {
	client, err := mgr.Client(identity, path)
	if err != nil {
		return nil, fmt.Errorf("gen: spawn %q: %w", identity, err)
	}
	resp, err := client.GetProviderSchema(ctx, &tfplugin6.GetProviderSchema_Request{})
	if err != nil {
		return nil, fmt.Errorf("gen: GetProviderSchema: %w", err)
	}
	if err := diagErr(resp.GetDiagnostics()); err != nil {
		return nil, err
	}

	var out []Resource
	for typeName, sch := range resp.GetResourceSchemas() {
		attrs, err := schemaAttrs(sch.GetBlock())
		if err != nil {
			return nil, fmt.Errorf("gen: resource %q: %w", typeName, err)
		}
		out = append(out, Resource{Type: typeName, Attrs: attrs})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out, nil
}

// schemaAttrs converts a schema block's attributes into the model, parsing each
// attribute's tftype (or recursing into a NestedType block).
func schemaAttrs(block *tfplugin6.Schema_Block) ([]Attr, error) {
	if block == nil {
		return nil, nil
	}
	var attrs []Attr
	for _, a := range block.GetAttributes() {
		attr := Attr{
			Name:      a.GetName(),
			Required:  a.GetRequired(),
			Optional:  a.GetOptional(),
			Computed:  a.GetComputed(),
			Sensitive: a.GetSensitive(),
		}
		if nt := a.GetNestedType(); nt != nil {
			nested, err := nestedAttrs(nt)
			if err != nil {
				return nil, fmt.Errorf("attr %q: %w", a.GetName(), err)
			}
			attr.Type = nestedObjectType(nested)
		} else {
			t, err := tftypes.ParseJSONType(a.GetType())
			if err != nil {
				return nil, fmt.Errorf("attr %q: parse type: %w", a.GetName(), err)
			}
			attr.Type = mapType(t)
		}
		attrs = append(attrs, attr)
	}
	sortAttrs(attrs)
	return attrs, nil
}

// nestedAttrs parses a nested object's attributes recursively.
func nestedAttrs(obj *tfplugin6.Schema_Object) ([]Attr, error) {
	var attrs []Attr
	for _, a := range obj.GetAttributes() {
		attr := Attr{
			Name:      a.GetName(),
			Required:  a.GetRequired(),
			Optional:  a.GetOptional(),
			Computed:  a.GetComputed(),
			Sensitive: a.GetSensitive(),
		}
		if nt := a.GetNestedType(); nt != nil {
			n, err := nestedAttrs(nt)
			if err != nil {
				return nil, err
			}
			attr.Type = nestedObjectType(n)
		} else {
			t, err := tftypes.ParseJSONType(a.GetType())
			if err != nil {
				return nil, err
			}
			attr.Type = mapType(t)
		}
		attrs = append(attrs, attr)
	}
	sortAttrs(attrs)
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
