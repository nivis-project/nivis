// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nivis-project/nivis/internal/ui"
)

// secretFiles reports, once, when a run CREATES a local file that can hold
// secrets from provider outputs: the state document and the outputs ledger beside
// it. The ledger is where sensitive outputs live in plaintext by design, because
// they are kept out of the world-readable Nix store (docs/IR-CONTRACT.md,
// "Sensitive values across the boundary"), and state carries provider outputs too.
//
// The notice exists because these land in the CALLER's working directory, where
// neither this project's ignore rules nor its documentation reach. It is the only
// part of that protection that reaches someone who reads neither.
//
// Creation is observed by comparing existence before and after the run rather than
// by reporting from the write sites: the same behaviour with no callback threaded
// through the state and ledger packages, and it covers the case where a remote
// backend still writes a local ledger.
type secretFiles struct {
	paths   []string
	existed map[string]bool
}

// watchSecretFiles records which of the run's local secret-bearing files already
// exist, so a later report names only the ones this run brought into being.
func watchSecretFiles(statePath string) *secretFiles {
	s := &secretFiles{
		paths:   []string{statePath, statePath + ".ledger"},
		existed: map[string]bool{},
	}
	for _, p := range s.paths {
		_, err := os.Stat(p)
		s.existed[p] = err == nil
	}
	return s
}

// created is the paths that did not exist when the run began and do now.
func (s *secretFiles) created() []string {
	var out []string
	for _, p := range s.paths {
		if s.existed[p] {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// report names the created files once. It never prints their CONTENTS, which
// would defeat its purpose, and it is informational: it cannot fail a run.
func (s *secretFiles) report(r ui.Renderer, level ui.Level) {
	if level < ui.LevelInfo {
		return
	}
	made := s.created()
	if len(made) == 0 {
		return
	}
	w := r.Writer()
	fmt.Fprintf(w, "note: this run created files that can contain secrets from provider outputs:\n")
	for _, p := range made {
		fmt.Fprintf(w, "        %s\n", p)
	}
	fmt.Fprintf(w, "      Do not commit them. To ignore them from your repository root:\n")
	for _, p := range ignorePatterns(made) {
		fmt.Fprintf(w, "        %s\n", p)
	}
}

// ignorePatterns turns the created paths into gitignore lines a caller can paste.
// A ledger gets the suffix rule, which follows it wherever --state put it; the
// state document is named as it actually is, since no general rule can cover an
// arbitrary --state path.
func ignorePatterns(made []string) []string {
	var out []string
	ledger := false
	for _, p := range made {
		if filepath.Ext(p) == ".ledger" {
			ledger = true
			continue
		}
		out = append(out, filepath.ToSlash(p))
	}
	if ledger {
		out = append(out, "*.ledger")
	}
	return out
}
