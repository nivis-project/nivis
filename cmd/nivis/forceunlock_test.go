// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// force-unlock against a backend that does not use locking (the local file store,
// which is what openStore falls back to with no flake in the test) reports a clear
// "nothing to force-unlock" message rather than crashing.
func TestForceUnlockNonLockerBackend(t *testing.T) {
	old := statePath
	statePath = filepath.Join(t.TempDir(), "s.json")
	defer func() { statePath = old }()

	cmd := forceUnlockCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader(""))
	cmd.SetArgs([]string{"--force"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error: the local store does not use locking")
	}
	if !strings.Contains(err.Error(), "does not use locking") {
		t.Errorf("error = %v, want it to explain there is no lock to clear", err)
	}
}
