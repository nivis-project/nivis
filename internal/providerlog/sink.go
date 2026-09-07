// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package providerlog

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

// maxDistinctNotes bounds the collapse bookkeeping. Reaching it is REPORTED
// (see Summary): a silent cap would present a partial list as the complete set.
const maxDistinctNotes = 200

// Sink renders provider entries as notes, collapses repeats, and reports the
// repeat counts at the end of a run.
//
// One Sink serves a whole run; each spawned provider gets its own io.Writer from
// Writer(identity), which is what the plugin manager hands to hclog as its
// Output. Collapsing is therefore run-wide: a note emitted by two providers, or
// once per resource across a stack, is printed once.
//
// The first occurrence prints immediately rather than being held for a
// post-run block: during a long apply, a warning about the resource being
// created right now is worth more than a tidy summary later.
type Sink struct {
	w     io.Writer
	level Level
	color bool

	mu sync.Mutex
	// seen counts occurrences per fingerprint (subject + message), in first-seen
	// order via order.
	seen  map[string]*noteCount
	order []string
	// truncated records that the distinct-note cap was reached.
	truncated bool
	// pending holds a partial line per provider, since an io.Writer may be
	// handed a fragment.
	pending map[string]*bytes.Buffer
}

// noteCount is one distinct note and how often it occurred.
type noteCount struct {
	subject string
	message string
	level   string
	count   int
}

// NewSink builds a Sink writing to w at the given level. color enables the
// marker styling; callers pass the same rule the rest of the CLI uses (a TTY and
// no NO_COLOR), so piped output stays stable for scripts and tests.
func NewSink(w io.Writer, level Level, color bool) *Sink {
	return &Sink{
		w:       w,
		level:   level,
		color:   color,
		seen:    map[string]*noteCount{},
		pending: map[string]*bytes.Buffer{},
	}
}

// Writer returns the io.Writer for one provider's log stream. provider is the
// identity the manager spawned (e.g. "aws"), used as a note's subject when the
// entry names no resource — the entry itself cannot be trusted for this, since
// its @module may be the provider's inner module.
func (s *Sink) Writer(provider string) io.Writer {
	return &providerWriter{sink: s, provider: provider}
}

// providerWriter is the per-provider Output handed to hclog.
type providerWriter struct {
	sink     *Sink
	provider string
}

// Write accepts serialized entries, one per line, tolerating fragments: hclog
// writes a whole line per entry, but an io.Writer contract does not guarantee it.
func (p *providerWriter) Write(b []byte) (int, error) {
	p.sink.write(p.provider, b)
	return len(b), nil
}

// write buffers by provider and emits a note per complete line.
func (s *Sink) write(provider string, b []byte) {
	s.mu.Lock()
	buf, ok := s.pending[provider]
	if !ok {
		buf = &bytes.Buffer{}
		s.pending[provider] = buf
	}
	buf.Write(b)
	var lines []string
	for {
		line, err := buf.ReadString('\n')
		if err != nil {
			// Incomplete: put the remainder back and wait for the rest.
			buf.WriteString(line)
			break
		}
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	s.mu.Unlock()

	for _, line := range lines {
		s.Note(provider, []byte(line))
	}
}

// Note renders one serialized entry, printing it unless it is a repeat.
func (s *Sink) Note(provider string, line []byte) {
	if s.level == LevelOff {
		return
	}
	e := Decode(line)

	// At trace the entries are wanted unabridged: print the whole thing, and do
	// not collapse (a debugging session wants every occurrence, with its ids).
	if s.level.verbatim() {
		fmt.Fprintln(s.w, s.render(e, provider, true))
		return
	}

	subject := e.Subject(provider)
	fingerprint := subject + "\x00" + e.Message

	s.mu.Lock()
	if nc, ok := s.seen[fingerprint]; ok {
		nc.count++
		s.mu.Unlock()
		return // a repeat: counted, reported by Summary
	}
	if len(s.seen) >= maxDistinctNotes {
		s.truncated = true
		s.mu.Unlock()
		return
	}
	s.seen[fingerprint] = &noteCount{subject: subject, message: e.Message, level: e.Level, count: 1}
	s.order = append(s.order, fingerprint)
	s.mu.Unlock()

	fmt.Fprintln(s.w, s.render(e, provider, false))
}

// Summary reports the repeat counts collected during the run: one line per note
// that occurred more than once, plus a truncation notice if the distinct-note cap
// was reached. It is a no-op when nothing repeated.
func (s *Sink) Summary() {
	if s.level == LevelOff || s.level.verbatim() {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	var repeated []*noteCount
	for _, fp := range s.order {
		if nc := s.seen[fp]; nc != nil && nc.count > 1 {
			repeated = append(repeated, nc)
		}
	}
	sort.SliceStable(repeated, func(i, j int) bool { return repeated[i].count > repeated[j].count })

	if len(repeated) == 0 && !s.truncated {
		return
	}
	fmt.Fprintln(s.w)
	for _, nc := range repeated {
		fmt.Fprintf(s.w, "%s %s: %s (%d further occurrence(s) not shown)\n",
			s.marker(nc.level), nc.subject, nc.message, nc.count-1)
	}
	if s.truncated {
		fmt.Fprintf(s.w, "%s provider notes: more than %d distinct notes; the list above is truncated\n",
			s.marker("warn"), maxDistinctNotes)
	}
}

// render builds the line for one entry. verbatim appends the telemetry fields a
// note otherwise drops.
func (s *Sink) render(e Entry, provider string, verbatim bool) string {
	var b strings.Builder
	b.WriteString(s.marker(e.Level))
	b.WriteString(" ")
	b.WriteString(e.Subject(provider))
	b.WriteString(": ")
	b.WriteString(e.Message)

	// The `error` field is a CAUSE, not a failure: it is subordinated as a
	// detail and never labelled "error".
	if d := e.Detail(); d != "" {
		b.WriteString(" (detail: ")
		b.WriteString(d)
		b.WriteString(")")
	}
	if verbatim {
		if fields := e.telemetry(); len(fields) > 0 {
			b.WriteString(" [")
			b.WriteString(strings.Join(fields, " "))
			b.WriteString("]")
		}
	}
	return b.String()
}

// marker is the note's level marker. An error-level entry is marked as an error;
// everything else is a note, so a warning cannot be mistaken for a failure.
func (s *Sink) marker(level string) string {
	switch level {
	case "error":
		return s.paint(ansiRed, "provider error")
	case "trace", "debug":
		return s.paint(ansiDim, "provider debug")
	case "info":
		return s.paint(ansiDim, "provider info")
	default:
		return s.paint(ansiYellow, "provider note")
	}
}

// Colours match the CLI's change-type palette (cmd/nivis/output.go).
const (
	ansiYellow = "\x1b[38;2;220;190;90m"
	ansiRed    = "\x1b[38;2;220;100;90m"
	ansiDim    = "\x1b[2m"
	ansiReset  = "\x1b[0m"
)

func (s *Sink) paint(code, text string) string {
	if !s.color {
		return text
	}
	return code + text + ansiReset
}
