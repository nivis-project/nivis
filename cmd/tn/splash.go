// Copyright 2026 WeareTechnative B.V. and the terrae-nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// version is the displayed version (overridable at build time via -ldflags).
var version = "v1.0"

// Brand ANSI colours (24-bit truecolor) from docs/BRAND.md:
//
//	ember #F2632E (the one warm accent), ice blue #AECFE6, silver #C3D2DE.
const (
	ansiReset = "\x1b[0m"
	ansiEmber = "\x1b[38;2;242;99;46m"   // Volcanic Ember
	ansiIce   = "\x1b[38;2;174;207;230m" // Ice Blue
	ansiDim   = "\x1b[38;2;120;140;160m" // dim grey-blue
	ansiSnow  = "\x1b[1;38;2;245;250;253m"
)

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

// splash prints the branded Terrae Nivis splash to w. Colour is applied only
// when w is a colour-capable TTY; otherwise it is plain text.
func splash(w io.Writer) {
	c := colorEnabled(w)
	paint := func(code, s string) string {
		if !c {
			return s
		}
		return code + s + ansiReset
	}

	// ASCII peak + wordmark (handoff "10 · Command line").
	fmt.Fprintln(w)
	fmt.Fprintf(w, "          %s\n", paint(ansiEmber, "*"))
	fmt.Fprintf(w, "         %s        %s\n", paint(ansiIce, "/\\"), paint(ansiSnow, "TERRAE NIVIS"))
	fmt.Fprintf(w, "        %s       %s\n", paint(ansiIce, "/  \\"), paint(ansiDim, "infrastructure as nix code · "+version))
	fmt.Fprintf(w, "       %s\n", paint(ansiIce, "/____\\"))
	fmt.Fprintf(w, "      %s\n", paint(ansiIce, "/\\/\\/\\/\\"))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s plan · apply · destroy · refresh · state    (try %s)\n",
		paint(ansiEmber, "❯"), paint(ansiIce, "tn --help"))
	fmt.Fprintln(w)
}
