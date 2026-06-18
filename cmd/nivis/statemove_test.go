// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wearetechnative/nivis/internal/state"
)

// runPush drives the push command with the given stdin and args, returning the
// combined output and any error. statePath is the package-global the command's
// openStore() reads.
func runPush(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	cmd := pushCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// A push from a non-TTY (a string reader is not a terminal) without --force is
// refused, and state is left unchanged.
func TestPushNonInteractiveRequiresForce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.json")
	st, _ := state.Open(path)
	if err := st.Set(state.ResourceState{ID: "keep.it.here", Type: "t", Attrs: map[string]interface{}{}}); err != nil {
		t.Fatal(err)
	}
	old := statePath
	statePath = path
	defer func() { statePath = old }()

	out, err := runPush(t, `{"resources":{}}`)
	if err == nil {
		t.Fatal("expected a refusal without --force on non-interactive input")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error should tell the user to pass --force; got %v", err)
	}
	_ = out
	// state unchanged
	if _, ok, _ := st.Get("keep.it.here"); !ok {
		t.Error("state must be unchanged after a refused push")
	}
}

// push --force replaces the whole state from the input document.
func TestPushForceReplacesState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.json")
	st, _ := state.Open(path)
	if err := st.Set(state.ResourceState{ID: "old.x.y", Type: "t", Attrs: map[string]interface{}{}}); err != nil {
		t.Fatal(err)
	}
	old := statePath
	statePath = path
	defer func() { statePath = old }()

	doc := `{"resources":{"new.a.b":{"id":"new.a.b","type":"t","attrs":{}}}}`
	out, err := runPush(t, doc, "--force")
	if err != nil {
		t.Fatalf("push --force: %v\n%s", err, out)
	}
	if _, ok, _ := st.Get("new.a.b"); !ok {
		t.Error("new resource should be present after push --force")
	}
	if _, ok, _ := st.Get("old.x.y"); ok {
		t.Error("old resource should be gone after push --force")
	}
}

// push of malformed input fails and leaves state unchanged.
func TestPushRejectsMalformed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.json")
	st, _ := state.Open(path)
	_ = st.Set(state.ResourceState{ID: "keep.me.x", Type: "t", Attrs: map[string]interface{}{}})
	old := statePath
	statePath = path
	defer func() { statePath = old }()

	if _, err := runPush(t, "not a state document", "--force"); err == nil {
		t.Fatal("expected malformed push to error")
	}
	if _, ok, _ := st.Get("keep.me.x"); !ok {
		t.Error("state must survive a malformed push")
	}
}
