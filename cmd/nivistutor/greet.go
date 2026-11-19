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
		return code + s + ui.Reset
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s  %s\n", paint(ui.Accent, "●"), paint(ui.Bright, "Welcome to the Nivis Tutorials"))
	fmt.Fprintf(w, "     %s\n", paint(ui.Dim, "scaffold a tutorial, then run it with plain nivis · "+version))
	fmt.Fprintln(w)
}
