// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package ledger

import (
	"encoding/json"
	"strings"
	"testing"
)

// vars is omitted from the wire when empty (omitempty), so a run with no
// variables injects { phase, outputs } exactly as before this feature.
func TestVarsOmittedWhenEmpty(t *testing.T) {
	l := New()
	data, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "\"vars\"") {
		t.Errorf("empty vars should be omitted; got %s", data)
	}
}

// vars is marshaled when set, alongside phase and outputs.
func TestVarsMarshaledWhenSet(t *testing.T) {
	l := New()
	l.Phase = 2
	l.Vars = map[string]interface{}{"region": "eu-central-1", "count": float64(3)}
	data, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]interface{}
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	v, ok := back["vars"].(map[string]interface{})
	if !ok {
		t.Fatalf("vars missing or wrong shape: %s", data)
	}
	if v["region"] != "eu-central-1" || v["count"] != float64(3) {
		t.Errorf("vars round-trip: %#v", v)
	}
	if back["phase"] != float64(2) {
		t.Errorf("phase: %#v", back["phase"])
	}
}
