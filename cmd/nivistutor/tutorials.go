// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"embed"
	"io/fs"
	"sort"
	"strings"
)

// embeddedTutorials holds the starter directories baked into the binary, so
// nivistutor works offline and always writes files matching its own build. Each
// subdirectory of tutorials/ is one tutorial (its name is the directory name).
//
//go:embed all:tutorials
var embeddedTutorials embed.FS

// tutorial is one scaffold-able starter: a name (the directory) and a one-line
// summary read from the first heading of its README.
type tutorial struct {
	Name    string // directory name, e.g. "getting-started"
	Summary string // first markdown heading of README.md, for the menu
}

// nivisRefPlaceholder is the token in the embedded starter flake.nix that
// scaffolding rewrites to the nivis ref this build is pinned to (nivisRef()).
const nivisRefPlaceholder = "@NIVIS_REF@"

// tutorialsRoot is the embed path prefix.
const tutorialsRoot = "tutorials"

// listTutorials returns the embedded tutorials, sorted by name for a stable menu.
func listTutorials() ([]tutorial, error) {
	entries, err := fs.ReadDir(embeddedTutorials, tutorialsRoot)
	if err != nil {
		return nil, err
	}
	var out []tutorial
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		out = append(out, tutorial{
			Name:    e.Name(),
			Summary: readSummary(e.Name()),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// findTutorial returns the named tutorial, or ok=false if it is not embedded.
func findTutorial(name string) (tutorial, bool) {
	ts, err := listTutorials()
	if err != nil {
		return tutorial{}, false
	}
	for _, t := range ts {
		if t.Name == name {
			return t, true
		}
	}
	return tutorial{}, false
}

// readSummary returns the first markdown heading (without the leading '#') of a
// tutorial's README, used as the menu line. Empty if there is no README/heading.
func readSummary(name string) string {
	b, err := embeddedTutorials.ReadFile(tutorialsRoot + "/" + name + "/README.md")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		s := strings.TrimSpace(line)
		if strings.HasPrefix(s, "#") {
			return strings.TrimSpace(strings.TrimLeft(s, "#"))
		}
	}
	return ""
}
