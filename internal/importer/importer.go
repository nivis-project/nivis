// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package importer adopts existing infrastructure into nivis state: nothing is
// created, changed or destroyed, and the only thing written is state.
//
// `state push` can already write state documents by hand, so this earns its
// place only by refusing what it cannot do safely. Its governing rule is that a
// WRONG import must not be able to look like a right one, which is why it
// refuses in four cases rather than guessing (see Run).
//
// The flow per entry mirrors what a provider expects:
//
//	Import(type, id)   -> a stub, often little more than the identity
//	Read(the stub)     -> the provider's full view
//	seeded values      -> applied last, overriding the read
//
// Seeded values exist because some attributes are returned once at creation and
// never again. Nothing in the protocol marks them, so the manifest says which
// they are, and they go last because the read would otherwise overwrite them.
package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/nivis-project/nivis/internal/ir"
	"github.com/nivis-project/nivis/internal/plan"
	"github.com/nivis-project/nivis/internal/provider"
	"github.com/nivis-project/nivis/internal/state"
)

// Manager spawns provider clients. Mirrors the seam the other packages use.
type Manager interface {
	Client(name, source string, config map[string]interface{}) (provider.Client, error)
}

// Entry is one resource to adopt: the provider's own identifier for it, and any
// attribute values nivis must take on trust because the provider will not
// return them.
type Entry struct {
	ID   string                 `json:"id"`
	Seed map[string]interface{} `json:"seed,omitempty"`
}

// Manifest maps a nivis resource id to the object to adopt into it.
type Manifest struct {
	Imports map[string]Entry `json:"imports"`
}

// Result is what one manifest entry produced.
type Result struct {
	ID string
	// Imported is false when the entry was refused; Err then says why and what
	// to do about it.
	Imported bool
	// Seeded names the attributes taken on trust rather than read from the
	// provider, so a reader can tell what was asserted from what was observed.
	Seeded []string
	Err    error
}

// Summary is the whole run. A migration imports hundreds of resources at once,
// so the counts matter as much as the detail.
type Summary struct {
	Results  []Result
	Imported int
	Refused  int
}

// ParseManifest reads a manifest, refusing a malformed one before any provider
// is contacted.
func ParseManifest(data []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("import manifest is not valid JSON: %w", err)
	}
	if len(m.Imports) == 0 {
		return m, fmt.Errorf("import manifest declares no imports: expected an \"imports\" object mapping a nivis resource id to its provider import id")
	}
	for id, e := range m.Imports {
		if e.ID == "" {
			return m, fmt.Errorf("import manifest entry %q has no \"id\": the provider's own identifier for the object is required", id)
		}
	}
	return m, nil
}

// LoadManifest reads a manifest from disk. It reports whether any entry seeds
// attribute values, because those are credentials by definition and the file
// holding them should not be committed.
func LoadManifest(path string) (Manifest, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, false, fmt.Errorf("read import manifest %q: %w", path, err)
	}
	m, err := ParseManifest(data)
	if err != nil {
		return m, false, err
	}
	for _, e := range m.Imports {
		if len(e.Seed) > 0 {
			return m, true, nil
		}
	}
	return m, false, nil
}

// Run imports every entry in the manifest, in a deterministic order.
//
// It refuses, rather than guessing, in four cases. Each is one where continuing
// would write state that looks correct and is not:
//
//   - an id the configuration does not declare, which would be an entry nivis
//     never plans, applies or destroys, so the user believes something is
//     managed that nothing will touch;
//   - more than one resource returned for one import, because every nivis
//     address comes from the configuration and there is nowhere to put extras;
//   - a read that comes back empty, which means the import id identified
//     nothing. This must NOT reuse the reading elsewhere in nivis that an empty
//     read is an out-of-band deletion, because here it would create a duplicate
//     of something that may well exist;
//   - a resource already in state, which would be silently discarded.
//
// A refused entry writes nothing and does not stop the others: a run over two
// hundred resources that fails on three is resumable by fixing those three.
func Run(ctx context.Context, g *ir.Graph, mgr Manager, store state.Store, m Manifest) (*Summary, error) {
	sum := &Summary{}
	ids := make([]string, 0, len(m.Imports))
	for id := range m.Imports {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		res := importOne(ctx, g, mgr, store, id, m.Imports[id])
		sum.Results = append(sum.Results, res)
		if res.Imported {
			sum.Imported++
		} else {
			sum.Refused++
		}
	}
	return sum, nil
}

func importOne(
	ctx context.Context, g *ir.Graph, mgr Manager, store state.Store,
	id string, entry Entry,
) Result {
	fail := func(err error) Result { return Result{ID: id, Err: err} }

	node, declared := g.Nodes[id]
	if !declared || node.Resource.IsData {
		return fail(fmt.Errorf(
			"%q is not declared in the configuration.\n"+
				"Nivis only manages resources the configuration declares: an imported resource it does not know about would never be planned, applied or destroyed. Declare it first, then import", id))
	}
	if _, found, err := store.Get(id); err != nil {
		return fail(fmt.Errorf("read state for %q: %w", id, err))
	} else if found {
		return fail(fmt.Errorf(
			"%q is already in state.\n"+
				"Importing over it would discard what is there. Remove it with `nivis state rm %s` first if you mean to replace it", id, id))
	}

	prov, ok := g.Providers[node.Resource.Provider]
	if !ok {
		return fail(fmt.Errorf("provider %q not declared", node.Resource.Provider))
	}
	client, err := mgr.Client(node.Resource.Provider, prov.Source, prov.Config)
	if err != nil {
		return fail(fmt.Errorf("provider client for %q: %w", id, err))
	}
	rs, err := plan.SchemaFor(ctx, client, node.Resource.Type)
	if err != nil {
		return fail(fmt.Errorf("schema for %q: %w", id, err))
	}

	imp, err := client.Import(ctx, provider.ImportRequest{
		Schema: rs, TypeName: node.Resource.Type, ImportID: entry.ID,
	})
	if err != nil {
		return fail(fmt.Errorf("import %q as %q: %w", id, entry.ID, err))
	}
	if len(imp.Resources) != 1 {
		return fail(multiResourceError(id, entry.ID, imp.Resources))
	}

	// The import returns a stub. The read is what populates it.
	read, err := client.Read(ctx, provider.ReadRequest{
		Schema: rs, TypeName: node.Resource.Type, Stored: imp.Resources[0].Attrs,
	})
	if err != nil {
		return fail(fmt.Errorf("read %q after importing %q: %w", id, entry.ID, err))
	}
	if len(read.Attrs) == 0 {
		return fail(fmt.Errorf(
			"no object exists with import id %q for %q.\n"+
				"Only pre-existing objects can be imported. Check the id is correct and that it belongs to the provider configuration nivis is using", entry.ID, id))
	}

	// Seeded values go LAST: the read returns the provider's full view, which by
	// definition excludes attributes it never returns, so seeding earlier would
	// be overwritten.
	attrs := make(map[string]interface{}, len(read.Attrs)+len(entry.Seed))
	for k, v := range read.Attrs {
		attrs[k] = v
	}
	seeded := make([]string, 0, len(entry.Seed))
	for k, v := range entry.Seed {
		attrs[k] = v
		seeded = append(seeded, k)
	}
	sort.Strings(seeded)

	if err := store.Set(state.ResourceState{
		ID: id, Type: node.Resource.Type, SchemaVersion: rs.Version, Attrs: attrs,
	}); err != nil {
		return fail(fmt.Errorf("write state for %q: %w", id, err))
	}
	return Result{ID: id, Imported: true, Seeded: seeded}
}

// multiResourceError names what the provider returned, so the user can declare
// those resources rather than being told only that there were too many.
func multiResourceError(id, importID string, got []provider.ImportedResource) error {
	types := make([]string, 0, len(got))
	for _, r := range got {
		types = append(types, r.TypeName)
	}
	if len(got) == 0 {
		return fmt.Errorf(
			"the provider returned no resources for import id %q (%q).\n"+
				"Check the id is correct", importID, id)
	}
	return fmt.Errorf(
		"the provider returned %d resources for import id %q, and nivis can only adopt one into %q: %v.\n"+
			"Every nivis address comes from the configuration, so there is nowhere to put the others. Declare them and import each by its own id, or write their state directly with `nivis state push`",
		len(got), importID, id, types)
}
