// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"io"

	"github.com/nivis-project/nivis/internal/progress"
)

// plain reports one line per transition and never moves the cursor. It is the
// renderer for a pipe, a `dumb` terminal, a CI log, and any verbosity other than
// the default — and the fallback whenever cursor capability is not positively
// established. It carries the same information the live renderer does; only the
// presentation differs.
type plain struct{ base }

func (p *plain) Emit(e progress.Event) {
	line := p.line(e)
	if line == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintln(p.w, line)
}

// Writer hands out the underlying stream. There is no region to protect, so a
// note can be written directly — but it still goes through the renderer's lock,
// so a note cannot interleave with a line mid-write.
func (p *plain) Writer() io.Writer { return &lockedWriter{base: &p.base} }

// Suspend has nothing to tear down: a subprocess may write freely.
func (p *plain) Suspend(fn func() error) error { return fn() }

func (p *plain) Close() {}

// lockedWriter serialises another producer's writes against the renderer's.
type lockedWriter struct{ base *base }

func (l *lockedWriter) Write(b []byte) (int, error) {
	l.base.mu.Lock()
	defer l.base.mu.Unlock()
	return l.base.w.Write(b)
}

// Discard returns a renderer that reports nothing. It is for a code path with
// no user watching — shell completion, and tests that care only about the
// result — so that such a caller still goes through the Renderer contract
// rather than being handed a bare writer.
func Discard() Renderer {
	return &plain{base: base{w: io.Discard, level: LevelQuiet}}
}
