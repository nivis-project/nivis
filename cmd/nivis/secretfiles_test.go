// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

// The notice that names secret-bearing files a run CREATES in the working
// directory. It is the only part of that protection that reaches a user who reads
// neither our ignore rules nor our docs, so it is tested on what it says as well
// as on when it says it.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/ui"
)

// render captures what the notice writes at a level.
func render(t *testing.T, s *secretFiles, level ui.Level) string {
	t.Helper()
	var buf bytes.Buffer
	r := ui.New(&buf, level, ui.ColorNever)
	defer r.Close()
	s.report(r, level)
	return buf.String()
}

func touch(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// A run that creates both files names both, once, and says not to commit them.
func TestNoticeNamesCreatedFiles(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "nivis.state.json")

	s := watchSecretFiles(statePath)
	touch(t, statePath, `{"resources":{}}`)
	touch(t, statePath+".ledger", `{"outputs":{}}`)

	out := render(t, s, ui.LevelInfo)
	for _, want := range []string{statePath, statePath + ".ledger", "Do not commit"} {
		if !strings.Contains(out, want) {
			t.Errorf("notice should mention %q, got:\n%s", want, out)
		}
	}
	if n := strings.Count(out, "Do not commit"); n != 1 {
		t.Errorf("the notice should appear once, got %d times:\n%s", n, out)
	}
}

// Files that already existed are not reported: a routine second run is silent.
func TestNoticeSilentWhenFilesExisted(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "nivis.state.json")
	touch(t, statePath, `{"resources":{}}`)
	touch(t, statePath+".ledger", `{"outputs":{}}`)

	s := watchSecretFiles(statePath)
	// The run updates both, creating neither.
	touch(t, statePath, `{"resources":{"a":{}}}`)
	touch(t, statePath+".ledger", `{"outputs":{"a":{}}}`)

	if out := render(t, s, ui.LevelInfo); out != "" {
		t.Errorf("a run that created nothing should be silent, got:\n%s", out)
	}
}

// Only what was actually created is named: a pre-existing state file with a new
// ledger reports the ledger alone.
func TestNoticeNamesOnlyWhatWasCreated(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "nivis.state.json")
	touch(t, statePath, `{"resources":{}}`)

	s := watchSecretFiles(statePath)
	touch(t, statePath+".ledger", `{"outputs":{}}`)

	out := render(t, s, ui.LevelInfo)
	if !strings.Contains(out, statePath+".ledger") {
		t.Errorf("the created ledger should be named, got:\n%s", out)
	}
	// The state path is a prefix of the ledger path, so count lines instead.
	if n := strings.Count(out, ".ledger"); n == 0 {
		t.Errorf("expected the ledger, got:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == statePath {
			t.Errorf("the pre-existing state file should not be named, got:\n%s", out)
		}
	}
}

// A caller-chosen --state path is named as it actually is, since no ignore rule
// can cover an arbitrary path.
func TestNoticeNamesACustomStatePath(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "build", "stacks", "prod.json")

	s := watchSecretFiles(statePath)
	touch(t, statePath, `{"resources":{}}`)
	touch(t, statePath+".ledger", `{"outputs":{}}`)

	out := render(t, s, ui.LevelInfo)
	if !strings.Contains(out, statePath) {
		t.Errorf("the custom state path should be named, got:\n%s", out)
	}
	if !strings.Contains(out, "*.ledger") {
		t.Errorf("the ledger's suffix rule follows a custom path and should be offered, got:\n%s", out)
	}
}

// Quiet suppresses it, the way it suppresses the rest of Nivis's own narrative.
func TestNoticeSuppressedWhenQuiet(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "nivis.state.json")

	s := watchSecretFiles(statePath)
	touch(t, statePath, `{"resources":{}}`)

	if out := render(t, s, ui.LevelQuiet); out != "" {
		t.Errorf("quiet should suppress the notice, got:\n%s", out)
	}
	if out := render(t, s, ui.LevelInfo); out == "" {
		t.Error("the default level should report it")
	}
}

// The notice never carries a value from either file. It names paths; printing
// what it is warning about would defeat it.
func TestNoticeNeverPrintsContents(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "nivis.state.json")

	s := watchSecretFiles(statePath)
	const secret = "ghp_averysecretgithubtokenvalue"
	touch(t, statePath, `{"resources":{"a":{"attrs":{"token":"`+secret+`"}}}}`)
	touch(t, statePath+".ledger", `{"outputs":{"a":{"token":"`+secret+`"}}}`)

	out := render(t, s, ui.LevelInfo)
	if out == "" {
		t.Fatal("the notice should have been reported")
	}
	if strings.Contains(out, secret) {
		t.Errorf("the notice must never carry a value from the files, got:\n%s", out)
	}
	if strings.Contains(out, "token") {
		t.Errorf("the notice must not echo file contents at all, got:\n%s", out)
	}
}

// It is ANSI-free under the never-colour mode, like the rest of the output.
func TestNoticeIsPlainWithoutColour(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "nivis.state.json")

	s := watchSecretFiles(statePath)
	touch(t, statePath, `{"resources":{}}`)

	if out := render(t, s, ui.LevelInfo); strings.Contains(out, "\x1b[") {
		t.Errorf("the notice should carry no ANSI codes, got: %q", out)
	}
}

// Several writes across a run still produce ONE notice. A run rewrites state once
// per phase and the ledger after each, so a per-write notice would be noise.
func TestNoticeReportedOncePerRunNotPerWrite(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "nivis.state.json")

	s := watchSecretFiles(statePath)
	for i := 0; i < 4; i++ {
		touch(t, statePath, `{"resources":{}}`)
		touch(t, statePath+".ledger", `{"outputs":{}}`)
	}

	out := render(t, s, ui.LevelInfo)
	if n := strings.Count(out, "Do not commit"); n != 1 {
		t.Errorf("four writes should still yield one notice, got %d:\n%s", n, out)
	}
	if n := strings.Count(out, statePath+".ledger"); n != 1 {
		t.Errorf("the ledger should be named once, got %d:\n%s", n, out)
	}
}
