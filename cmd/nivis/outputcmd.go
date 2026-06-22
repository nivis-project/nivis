// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/wearetechnative/nivis/internal/phase"
)

// outputCmd prints a stack's declared outputs (the `outputs` arg to toIR),
// resolved against current state. No name: all as `name = value` lines (or a
// JSON object with --json). A name: just that output's value (or its JSON).
func outputCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "output [name]",
		Short: "Print the stack's declared outputs (resolved from state)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			mgr := newManager()
			defer mgr.Close()
			l, err := newLedger()
			if err != nil {
				return err
			}
			d := &phase.Driver{Eval: evaluator(), Manager: mgr, Store: store, Ledger: l}

			outputs, err := d.ResolveOutputs(cmd.Context())
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()

			// single named output
			if len(args) == 1 {
				name := args[0]
				v, ok := outputs[name]
				if !ok {
					return fmt.Errorf("%q is not a declared output", name)
				}
				if asJSON {
					return writeJSON(w, v)
				}
				fmt.Fprintln(w, formatValue(v))
				return nil
			}

			// all outputs
			if asJSON {
				return writeJSON(w, outputs)
			}
			if len(outputs) == 0 {
				fmt.Fprintln(w, "No outputs declared.")
				return nil
			}
			names := make([]string, 0, len(outputs))
			for n := range outputs {
				names = append(names, n)
			}
			sort.Strings(names)
			for _, n := range names {
				fmt.Fprintf(w, "%s = %s\n", n, formatValue(outputs[n]))
			}
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "print outputs as a JSON object (or the single value)")
	return c
}

// formatValue renders an output value for human output: a string as-is, anything
// else (number/bool/list/map) as compact JSON.
func formatValue(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

func writeJSON(w interface{ Write([]byte) (int, error) }, v interface{}) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}
