// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"embed"
	"io/fs"
	"sort"
	"strconv"
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

// listTutorials returns the embedded tutorials in a controlled, deterministic
// order (see lessTutorial): getting-started first, then features-<version>
// newest-first, then any others alphabetically.
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
	sort.Slice(out, func(i, j int) bool { return lessTutorial(out[i].Name, out[j].Name) })
	return out, nil
}

const gettingStarted = "getting-started"

// featuresPrefix marks a per-release feature tutorial: features-<version>.
const featuresPrefix = "features-"

// tutorialRank groups tutorials so getting-started sorts before features
// tutorials, which sort before everything else.
func tutorialRank(name string) int {
	switch {
	case name == gettingStarted:
		return 0
	case strings.HasPrefix(name, featuresPrefix):
		return 1
	default:
		return 2
	}
}

// lessTutorial orders the tutorial menu: getting-started first, then
// features-<version> newest version first (numeric, so 0.10 > 0.5 > 0.4), then
// any other tutorials alphabetically. Within the same rank and (for features) the
// same version, names break ties alphabetically for stability.
func lessTutorial(a, b string) bool {
	ra, rb := tutorialRank(a), tutorialRank(b)
	if ra != rb {
		return ra < rb
	}
	if ra == 1 { // both features-<version>: newest version first
		va := parseVersion(strings.TrimPrefix(a, featuresPrefix))
		vb := parseVersion(strings.TrimPrefix(b, featuresPrefix))
		if c := compareVersion(va, vb); c != 0 {
			return c > 0 // greater version sorts earlier
		}
	}
	return a < b
}

// parseVersion splits a dotted version ("0.4", "0.10.1") into numeric components.
// Non-numeric components compare as 0, so a malformed version sorts low but never
// panics.
func parseVersion(v string) []int {
	parts := strings.Split(v, ".")
	out := make([]int, len(parts))
	for i, p := range parts {
		n, _ := strconv.Atoi(p)
		out[i] = n
	}
	return out
}

// compareVersion returns >0 if a > b, <0 if a < b, 0 if equal, comparing
// component by component (missing components count as 0).
func compareVersion(a, b []int) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		var ai, bi int
		if i < len(a) {
			ai = a[i]
		}
		if i < len(b) {
			bi = b[i]
		}
		if ai != bi {
			if ai > bi {
				return 1
			}
			return -1
		}
	}
	return 0
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
