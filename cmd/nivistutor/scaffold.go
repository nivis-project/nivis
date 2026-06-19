// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// scaffoldOptions controls how a tutorial is written to disk.
type scaffoldOptions struct {
	// Dir is the target directory the starter files are written into. It is
	// created if missing.
	Dir string
	// Force allows overwriting existing files. When false, scaffolding refuses
	// (and writes nothing) if any target file already exists.
	Force bool
	// NivisRef is substituted for the @NIVIS_REF@ placeholder in flake.nix, so a
	// scaffolded project pins the nivis input to this build's release. Empty
	// leaves the placeholder in place (the README tells the user to fill it in).
	NivisRef string
}

// scaffold writes the named tutorial's starter files into opts.Dir. It returns
// the relative paths written (sorted), or an error. With Force=false it first
// checks every destination and fails atomically (writing nothing) if any exists,
// so a re-run never half-overwrites a user's edits.
func scaffold(name string, opts scaffoldOptions) ([]string, error) {
	if _, ok := findTutorial(name); !ok {
		return nil, fmt.Errorf("unknown tutorial %q (see `nivistutor --list`)", name)
	}
	srcRoot := tutorialsRoot + "/" + name

	// Collect the source files (relative to srcRoot) up front.
	var rels []string
	err := fs.WalkDir(embeddedTutorials, srcRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(srcRoot, p)
		if rerr != nil {
			return rerr
		}
		rels = append(rels, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(rels)

	// No-clobber: unless --force, refuse if any destination already exists.
	if !opts.Force {
		var clash []string
		for _, rel := range rels {
			if _, err := os.Stat(filepath.Join(opts.Dir, filepath.FromSlash(rel))); err == nil {
				clash = append(clash, rel)
			}
		}
		if len(clash) > 0 {
			return nil, fmt.Errorf("refusing to overwrite existing file(s) in %s: %v (use --force to overwrite)", opts.Dir, clash)
		}
	}

	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return nil, err
	}

	var written []string
	for _, rel := range rels {
		data, rerr := embeddedTutorials.ReadFile(srcRoot + "/" + rel)
		if rerr != nil {
			return written, rerr
		}
		if opts.NivisRef != "" {
			data = bytes.ReplaceAll(data, []byte(nivisRefPlaceholder), []byte(opts.NivisRef))
		}
		dst := filepath.Join(opts.Dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return written, err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return written, err
		}
		written = append(written, rel)
	}
	return written, nil
}
