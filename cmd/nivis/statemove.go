// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// pullCmd writes the whole state document (the backend snapshot) to stdout, or
// to --out. It is the read half of the document-level seam a remote backend
// reuses; the output re-pushes to reproduce the state exactly.
func pullCmd() *cobra.Command {
	var outFile string
	c := &cobra.Command{
		Use:   "pull",
		Short: "Write the whole state document to stdout (or --out)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := openStore()
			if err != nil {
				return err
			}
			snap, err := store.Snapshot()
			if err != nil {
				return err
			}
			if outFile != "" {
				if err := os.WriteFile(outFile, snap, 0o600); err != nil {
					return fmt.Errorf("write %q: %w", outFile, err)
				}
				return nil
			}
			_, err = cmd.OutOrStdout().Write(snap)
			return err
		},
	}
	c.Flags().StringVar(&outFile, "out", "", "write the snapshot to this file instead of stdout")
	return c
}

// pushCmd replaces the whole state from stdin (or --in). Because it overwrites
// the state of record, it confirms first (reporting incoming vs current counts),
// and requires --force to skip the prompt. When stdin is not a TTY (a pipe), the
// prompt cannot be answered, so --force is required and the command refuses
// otherwise: a scripted push is always explicit.
func pushCmd() *cobra.Command {
	var inFile string
	var force, yes bool
	c := &cobra.Command{
		Use:   "push",
		Short: "Replace the whole state from stdin (or --in); requires --force when non-interactive",
		RunE: func(cmd *cobra.Command, _ []string) error {
			force = force || yes // --yes is an alias for --force
			store, err := openStore()
			if err != nil {
				return err
			}

			var data []byte
			interactive := false
			if inFile != "" {
				data, err = os.ReadFile(inFile)
				if err != nil {
					return fmt.Errorf("read %q: %w", inFile, err)
				}
			} else {
				interactive = term.IsTerminal(int(os.Stdin.Fd()))
				data, err = io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("read stdin: %w", err)
				}
			}

			// counts for the confirmation message (incoming vs current).
			incoming, err := countResources(data)
			if err != nil {
				return err
			}
			current, err := store.List()
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if !force {
				if !interactive {
					return fmt.Errorf("push replaces all state (%d resource(s)) with the input (%d resource(s)); "+
						"rerun with --force to confirm (required for non-interactive input)", len(current), incoming)
				}
				fmt.Fprintf(out, "This replaces all state (%d resource(s)) with the input (%d resource(s)).\nProceed? [y/N] ", len(current), incoming)
				if !confirmed(cmd.InOrStdin()) {
					fmt.Fprintln(out, "Aborted; state unchanged.")
					return nil
				}
			}

			if err := store.Restore(data); err != nil {
				return err
			}
			fmt.Fprintf(out, "Replaced state with %d resource(s).\n", incoming)
			return nil
		},
	}
	c.Flags().StringVar(&inFile, "in", "", "read the state document from this file instead of stdin")
	c.Flags().BoolVar(&force, "force", false, "replace state without confirmation (required for non-interactive input)")
	c.Flags().BoolVar(&yes, "yes", false, "alias for --force")
	return c
}

// countResources reports how many resources a state-document blob contains,
// erroring (before any write) if it is not a valid state document.
func countResources(data []byte) (int, error) {
	var doc struct {
		Resources map[string]json.RawMessage `json:"resources"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return 0, fmt.Errorf("input is not a valid state document: %w", err)
	}
	return len(doc.Resources), nil
}

// confirmed reads one line and returns true for an affirmative answer.
func confirmed(r io.Reader) bool {
	line, _ := bufio.NewReader(r).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}
