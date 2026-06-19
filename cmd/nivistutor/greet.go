// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// Brand ANSI colours (24-bit truecolor) from docs/BRAND.md, matching the nivis
// splash: ember #F2632E (the one warm accent), ice blue #AECFE6.
const (
	ansiReset = "\x1b[0m"
	ansiEmber = "\x1b[38;2;242;99;46m"   // Volcanic Ember
	ansiIce   = "\x1b[38;2;174;207;230m" // Ice Blue
	ansiDim   = "\x1b[38;2;120;140;160m" // dim grey-blue
	ansiSnow  = "\x1b[1;38;2;245;250;253m"
)

// colorEnabled reports whether ANSI colour should be used: a TTY and no NO_COLOR
// (https://no-color.org).
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

// greet prints the branded welcome, consistent with the nivis splash. Colour is
// applied only on a colour-capable TTY.
func greet(w io.Writer) {
	c := colorEnabled(w)
	paint := func(code, s string) string {
		if !c {
			return s
		}
		return code + s + ansiReset
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s  %s\n", paint(ansiEmber, "*"), paint(ansiSnow, "Welcome to the Nivis Tutorials"))
	fmt.Fprintf(w, "     %s\n", paint(ansiDim, "scaffold a tutorial, then run it with plain nivis · "+version))
	fmt.Fprintln(w)
}
