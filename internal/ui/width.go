// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"io"
	"os"

	"golang.org/x/term"
)

// terminalWidth reports w's width in columns, or 0 when it is not a terminal or
// the width cannot be determined. Knowing the width is what lets a long resource
// id be elided in the middle rather than wrapped — a wrap splits an id across
// lines and is the main reason a long apply transcript becomes unreadable.
func terminalWidth(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok {
		return 0
	}
	cols, _, err := term.GetSize(int(f.Fd()))
	if err != nil {
		return 0
	}
	return cols
}
