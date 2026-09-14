// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/nivis-project/nivis/internal/phase"
	"github.com/nivis-project/nivis/internal/ui"
)

// The three output channels, and what belongs to each:
//
//   - stdout — THE RESULT: the change list and the final summary. Byte-identical
//     with and without a terminal, so redirecting it yields what a pipeline
//     would see. Rendered from the engines' return values by code that does not
//     know a renderer exists, so a renderer bug cannot corrupt it.
//   - stderr — THE NARRATIVE: progress, provider notes, Nix subprocess output.
//     Durable; it survives redirection, differing only in carrying no colour.
//   - stderr, terminal only — THE LIVE REGION: rewritten in place, erased before
//     the run ends, never present in redirected output.
//
// Content that reports what HAPPENED is narrative; content that reports what
// RESULTED is the result. Taking the state lock is narrative.

// newRenderer builds the run's renderer on the command's stderr. It is the sole
// owner of that stream: anything else with something to print goes through
// renderer.Writer or renderer.Suspend.
func newRenderer(stderr io.Writer) (ui.Renderer, error) {
	level, err := resolveLogLevel()
	if err != nil {
		return nil, err
	}
	mode, ok := ui.ParseColorMode(colorMode)
	if !ok {
		return nil, fmt.Errorf("unsupported --color %q (accepted: %s)",
			colorMode, joinOr(ui.ColorModeNames()))
	}
	return ui.New(stderr, level, mode), nil
}

// resolveLogLevel reads the verbosity from the flag, falling back to NIVIS_LOG
// and then the default. An explicit flag wins over the environment, so a CI
// system can set the variable once without preventing a one-off override.
func resolveLogLevel() (ui.Level, error) {
	if logLevel != "" {
		l, ok := ui.ParseLevel(logLevel)
		if !ok {
			return ui.DefaultLevel, fmt.Errorf("unsupported --log-level %q (accepted: %s)",
				logLevel, joinOr(ui.LevelNames()))
		}
		return l, nil
	}
	if v, ok := os.LookupEnv("NIVIS_LOG"); ok && v != "" {
		l, ok := ui.ParseLevel(v)
		if !ok {
			return ui.DefaultLevel, fmt.Errorf("unsupported NIVIS_LOG %q (accepted: %s)",
				v, joinOr(ui.LevelNames()))
		}
		return l, nil
	}
	return ui.DefaultLevel, nil
}

// terminalFor builds the subprocess-terminal policy for a renderer at the
// resolved level: Nix's own output is passed through only at the levels where
// the live region is off, so the two can never fight over the cursor.
func terminalFor(r ui.Renderer, level ui.Level) phase.Terminal {
	t := phase.Terminal{Suspend: r.Suspend}
	if level.PassThroughSubprocessOutput() {
		t.Stderr = r.Writer()
	}
	return t
}

// evalWith builds the run's evaluator with a subprocess-terminal policy
// attached, so Nix's own output can reach the user at the levels that pass it
// through. The evaluation itself is still memoized per ledger (see evalcache.go).
func evalWith(term phase.Terminal) phase.NixEvaluator {
	return runEvalTerm(term)
}

func joinOr(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	}
	out := ""
	for i, n := range names {
		switch {
		case i == 0:
			out = n
		case i == len(names)-1:
			out += " or " + n
		default:
			out += ", " + n
		}
	}
	return out
}

// testRenderer builds a plain renderer over w, for tests that want to inspect
// the narrative. Colour is off so assertions see plain text.
func testRenderer(w io.Writer) ui.Renderer { return ui.New(w, ui.LevelInfo, ui.ColorNever) }

// rendererFor builds the renderer for a command that has a user watching. Only
// a code path with NO user — shell completion — should use ui.Discard(): a
// discarding renderer silently swallows narrative the user is entitled to, such
// as openStore announcing that --backend=local overrode a declared backend.
func rendererFor(cmd *cobra.Command) (ui.Renderer, error) { return newRenderer(cmd.ErrOrStderr()) }
