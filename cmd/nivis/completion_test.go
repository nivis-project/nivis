// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/wearetechnative/nivis/internal/state"
)

// stateIDs completes to the resource ids in the state file, with NoFileComp so a
// shell never falls back to filename completion for a resource id.
func TestStateIDsCompletesFromStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st, err := state.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"aws.aws_s3_bucket.demo", "aws.aws_instance.web"} {
		if err := st.Set(state.ResourceState{ID: id, Type: "x", Attrs: map[string]interface{}{}}); err != nil {
			t.Fatal(err)
		}
	}

	// stateIDs reads the package-global statePath via openStore().
	old := statePath
	statePath = path
	defer func() { statePath = old }()

	ids, directive := stateIDs(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", directive)
	}
	got := map[string]bool{}
	for _, id := range ids {
		got[id] = true
	}
	if !got["aws.aws_s3_bucket.demo"] || !got["aws.aws_instance.web"] {
		t.Errorf("completions = %v, want both ids", ids)
	}
}

// A missing state file yields no completions and NoFileComp (not filenames).
func TestStateIDsEmptyWhenNoState(t *testing.T) {
	old := statePath
	statePath = filepath.Join(t.TempDir(), "does-not-exist.json")
	defer func() { statePath = old }()

	ids, directive := stateIDs(nil, nil, "")
	if len(ids) != 0 {
		t.Errorf("expected no completions for a missing store, got %v", ids)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", directive)
	}
}

// stateIDs only completes the first positional arg (a single id).
func TestStateIDsNoSecondArg(t *testing.T) {
	ids, directive := stateIDs(nil, []string{"already-have-one"}, "")
	if len(ids) != 0 {
		t.Errorf("expected no completions for a second arg, got %v", ids)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", directive)
	}
}
