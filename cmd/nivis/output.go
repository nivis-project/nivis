// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
)

// Change-type ANSI colours (24-bit truecolor), complementing splash.go's brand
// palette. Green/yellow/red carry the conventional plan-diff meaning.
const (
	ansiGreen  = "\x1b[38;2;90;200;120m" // create
	ansiYellow = "\x1b[38;2;220;190;90m" // update
	ansiRed    = "\x1b[38;2;220;100;90m" // destroy / replace (destroy half)
)

// output renders plan/apply/destroy lines, colourising by change type only when
// color is true. The markers and text are identical either way, so piped/NO_COLOR
// output (color=false) is stable for scripts and tests. The writer is the
// command's writer (capturable), not the process stdout.
type output struct {
	w     io.Writer
	color bool
}

// newOutput builds an output for w, enabling color per the shared colorEnabled
// rule (a TTY and no NO_COLOR).
func newOutput(w io.Writer) *output {
	return &output{w: w, color: colorEnabled(w)}
}

func (o *output) paint(code, s string) string {
	if !o.color {
		return s
	}
	return code + s + ansiReset
}

func (o *output) printf(format string, a ...interface{}) {
	fmt.Fprintf(o.w, format, a...)
}

// node prints one resource/datasource line: a colored marker + the id and type.
// The marker (and its meaning) is identical in plain mode; only the color differs.
//
//   - create   (green)
//     ~    update   (yellow)
//     -/+  replace  (red + green)
//   - destroy  (red)
//     =    no-op    (dim)
//     r    read     (dim; a datasource read, not a create)
func (o *output) create(id, typ string) { o.node(o.paint(ansiGreen, "+"), id, typ) }
func (o *output) update(id, typ string) { o.node(o.paint(ansiYellow, "~"), id, typ) }
func (o *output) replace(id, typ string) {
	o.node(o.paint(ansiRed, "-")+o.paint(ansiGreen, "/+"), id, typ)
}
func (o *output) destroy(id, typ string) { o.node(o.paint(ansiRed, "-"), id, typ) }
func (o *output) noop(id, typ string)    { o.node(o.paint(ansiDim, "="), id, typ) }
func (o *output) read(id, typ string)    { o.node(o.paint(ansiDim, "r"), id, typ) }

func (o *output) node(marker, id, typ string) {
	if typ != "" {
		o.printf("  %s %s (%s)\n", marker, id, typ)
	} else {
		o.printf("  %s %s\n", marker, id)
	}
}

// phaseHeading prints a "Phase N" group header (ice-blue on a TTY).
func (o *output) phaseHeading(n int) {
	o.printf("%s\n", o.paint(ansiIce, fmt.Sprintf("Phase %d", n)))
}
