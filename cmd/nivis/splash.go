// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"

	"github.com/nivis-project/nivis/internal/ui"
)

// version is the displayed version. The single source of truth is the top-level
// VERSION file: release builds (the Nix flake and goreleaser) inject it via
// `-ldflags -X main.version=<v>`. A plain `go build` with no ldflags reports
// "dev" rather than a stale hardcoded number.
var version = "dev"

const tagline = "Terraform providers as first-class Nix values"

// colorEnabled reports whether ANSI colour should be used: a TTY and no NO_COLOR.
// (https://no-color.org)
func colorEnabled(w io.Writer) bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// splash prints the branded Nivis splash to w. Colour is applied only
// when w is a colour-capable TTY; otherwise it is plain text.
func splash(w io.Writer) {
	c := colorEnabled(w)
	paint := func(code, s string) string {
		if !c {
			return s
		}
		return code + s + ui.Reset
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s  %s  %s\n", paint(ui.Accent, "●"), paint(ui.Bright, "Nivis"), paint(ui.Dim, tagline+" · "+version))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s plan · apply · destroy · refresh · state · gen    (try %s)\n",
		paint(ui.Accent, "❯"), paint(ui.Name, "nivis --help"))
	fmt.Fprintln(w)
}
