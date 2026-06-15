// Copyright 2026 WeareTechnative B.V. and the terrae-nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package ledger is the outputs ledger the phased-eval loop accumulates and
// injects into each Nix re-evaluation (docs/IR-CONTRACT.md "Outputs ledger").
// It is the carrier of apply-time provider outputs back into Nix.
package ledger

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/wearetechnative/terrae-nivis/internal/graph"
)

// Ledger is the contract's { phase, outputs } shape.
type Ledger struct {
	Phase   int                               `json:"phase"`
	Outputs map[string]map[string]interface{} `json:"outputs"`
}

// New returns an empty phase-0 ledger.
func New() *Ledger {
	return &Ledger{Phase: 0, Outputs: map[string]map[string]interface{}{}}
}

// Load reads a ledger from a JSON file. A missing file yields an empty ledger.
func Load(path string) (*Ledger, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return New(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("ledger: read %q: %w", path, err)
	}
	l := New()
	if len(data) > 0 {
		if err := json.Unmarshal(data, l); err != nil {
			return nil, fmt.Errorf("ledger: parse %q: %w", path, err)
		}
		if l.Outputs == nil {
			l.Outputs = map[string]map[string]interface{}{}
		}
	}
	return l, nil
}

// Save writes the ledger to path with mode 0600. The ledger may carry sensitive
// outputs, so it must never be world-readable and never land in the Nix store
// (docs/IR-CONTRACT.md "Sensitive values"). Written atomically (temp+rename),
// with the temp file also created 0600.
func (l *Ledger) Save(path string) error {
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return fmt.Errorf("ledger: marshal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("ledger: write temp: %w", err)
	}
	// Enforce 0600 even if umask or a pre-existing temp altered it.
	if err := os.Chmod(tmp, 0o600); err != nil {
		return fmt.Errorf("ledger: chmod temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("ledger: rename: %w", err)
	}
	return os.Chmod(path, 0o600)
}

// Append records a resource's computed outputs (merging into any existing entry).
func (l *Ledger) Append(id string, attrs map[string]interface{}) {
	if l.Outputs[id] == nil {
		l.Outputs[id] = map[string]interface{}{}
	}
	for k, v := range attrs {
		l.Outputs[id][k] = v
	}
}

// Known reports whether <id>.<attr> is present in the ledger.
func (l *Ledger) Known(id, attr string) bool {
	a, ok := l.Outputs[id]
	if !ok {
		return false
	}
	_, ok = a[attr]
	return ok
}

// Has reports whether any output is recorded for a resource id.
func (l *Ledger) Has(id string) bool {
	a, ok := l.Outputs[id]
	return ok && len(a) > 0
}

// ToGraphOutputs adapts the ledger for graph.ResolveTFTF.
func (l *Ledger) ToGraphOutputs() graph.Outputs {
	out := make(graph.Outputs, len(l.Outputs))
	for id, attrs := range l.Outputs {
		out[id] = attrs
	}
	return out
}
