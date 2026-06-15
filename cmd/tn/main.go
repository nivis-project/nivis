// Copyright 2026 WeareTechnative B.V. and the terrae-nivis authors
// SPDX-License-Identifier: Apache-2.0

// Command tn is the terrae-nivis executor CLI: plan/apply/destroy/refresh/state over a
// Nix flake that exposes terraeNivis.plan. Pure orchestration; providers are spawned
// from the IR's provider source paths.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/wearetechnative/terrae-nivis/internal/destroy"
	"github.com/wearetechnative/terrae-nivis/internal/ir"
	"github.com/wearetechnative/terrae-nivis/internal/ledger"
	"github.com/wearetechnative/terrae-nivis/internal/phase"
	"github.com/wearetechnative/terrae-nivis/internal/plugin"
	"github.com/wearetechnative/terrae-nivis/internal/refresh"
	"github.com/wearetechnative/terrae-nivis/internal/registry"
	"github.com/wearetechnative/terrae-nivis/internal/state"
)

var (
	flakeRef  string
	statePath string
	target    string
	attr      string
)

func main() {
	var showVersion bool
	root := &cobra.Command{
		Use:   "tn",
		Short: "Terrae Nivis — Infrastructure as Nix Code",
		Long: "Terrae Nivis — Infrastructure as Nix Code.\n\n" +
			"A Nix-native infrastructure tool where Terraform/OpenTofu provider\n" +
			"resources are first-class Nix values. (Formerly nixform.)",
		// We print runtime failures ourselves as a clean `error:` line; don't let
		// cobra print the error a second time.
		SilenceErrors: true,
		// Silence usage only AFTER flags parse successfully: flag/argument misuse
		// happens before this hook and still shows usage, but a RunE (runtime)
		// failure does not dump the usage block.
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			cmd.SilenceUsage = true
		},
		// With no subcommand, print the branded splash (or just the version).
		Run: func(cmd *cobra.Command, _ []string) {
			if showVersion {
				fmt.Fprintln(cmd.OutOrStdout(), "tn (Terrae Nivis) "+version)
				return
			}
			splash(cmd.OutOrStdout())
		},
	}
	root.PersistentFlags().StringVar(&flakeRef, "flake", ".", "flake reference exposing terraeNivis.plan")
	root.PersistentFlags().StringVar(&statePath, "state", "./terrae-nivis.state.json", "path to the local state file")
	root.PersistentFlags().StringVar(&attr, "attr", "terraeNivis.plan", "flake attribute to evaluate")
	root.PersistentFlags().StringVar(&target, "target", "", "restrict the operation to a single resource id")
	root.Flags().BoolVar(&showVersion, "version", false, "print version and exit")

	root.AddCommand(planCmd(), applyCmd(), destroyCmd(), refreshCmd(), stateCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// evaluator builds a real Nix evaluator from the flags.
func evaluator() phase.NixEval {
	return phase.NixEval{FlakeRef: flakeRef, Attr: attr, WorkDir: ""}
}

// newManager builds a plugin manager with the registry resolver attached, so a
// provider `source` that is a registry address (e.g. "hashicorp/aws") is
// fetched, verified, and cached before spawn; a filesystem path is used directly.
func newManager() *plugin.Manager {
	return plugin.NewManager().WithResolver(registry.New(""))
}

// phase0Graph evaluates the plan once (empty ledger) and ingests it, for the
// destroy/refresh engines which need the resource set + providers.
func phase0Graph(ctx context.Context) (*ir.Graph, error) {
	irJSON, err := evaluator().Eval(ctx, ledger.New())
	if err != nil {
		return nil, err
	}
	return ir.IngestIR(irJSON)
}

func openStore() (state.Store, error) { return state.Open(statePath) }

func planCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "plan",
		Short: "Evaluate the configuration and show what would be applied",
		RunE: func(cmd *cobra.Command, _ []string) error {
			// A dry phase-0 ingest + report; the full plan is part of apply for
			// the PoC (the phase loop plans each resource before applying).
			g, err := phase0Graph(cmd.Context())
			if err != nil {
				return err
			}
			for _, id := range g.Order {
				fmt.Printf("+ %s (%s)\n", id, g.Nodes[id].Resource.Type)
			}
			fmt.Printf("\n%d resource(s) to resolve across phases. Run `tn apply`.\n", len(g.Order))
			return nil
		},
	}
}

func applyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "apply",
		Short: "Resolve and apply the configuration to a fixpoint",
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := openStore()
			if err != nil {
				return err
			}
			mgr := newManager()
			defer mgr.Close()
			d := &phase.Driver{
				Eval:       evaluator(),
				Manager:    mgr,
				Store:      store,
				Ledger:     ledger.New(),
				LedgerPath: statePath + ".ledger",
			}
			res, err := d.Run(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Printf("Applied %d resource(s) across %d phase(s):\n", len(res.Applied), res.AppliedPhases)
			for _, id := range res.Applied {
				fmt.Printf("  ✓ %s\n", id)
			}
			return nil
		},
	}
}

func destroyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "destroy",
		Short: "Destroy applied resources in reverse dependency order",
		RunE: func(cmd *cobra.Command, _ []string) error {
			g, err := phase0Graph(cmd.Context())
			if err != nil {
				return err
			}
			store, err := openStore()
			if err != nil {
				return err
			}
			mgr := newManager()
			defer mgr.Close()
			res, err := destroy.Run(cmd.Context(), g, mgr, store, destroy.Options{Target: target})
			if err != nil {
				return err
			}
			fmt.Printf("Destroyed %d resource(s):\n", len(res.Destroyed))
			for _, id := range res.Destroyed {
				fmt.Printf("  - %s\n", id)
			}
			return nil
		},
	}
}

func refreshCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Reconcile state with the providers (ReadResource), no changes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			g, err := phase0Graph(cmd.Context())
			if err != nil {
				return err
			}
			store, err := openStore()
			if err != nil {
				return err
			}
			mgr := newManager()
			defer mgr.Close()
			res, err := refresh.Run(cmd.Context(), g, mgr, store)
			if err != nil {
				return err
			}
			fmt.Printf("Refreshed %d resource(s).\n", len(res.Refreshed))
			return nil
		},
	}
}

func stateCmd() *cobra.Command {
	c := &cobra.Command{Use: "state", Short: "Inspect and manage local state"}

	c.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List resource ids in state",
		RunE: func(_ *cobra.Command, _ []string) error {
			store, err := openStore()
			if err != nil {
				return err
			}
			items, err := store.List()
			if err != nil {
				return err
			}
			for _, rs := range items {
				fmt.Println(rs.ID)
			}
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show a resource's stored attributes",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			store, err := openStore()
			if err != nil {
				return err
			}
			rs, ok, err := store.Get(args[0])
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("no state for %q", args[0])
			}
			fmt.Printf("%s (%s)\n", rs.ID, rs.Type)
			for k, v := range rs.Attrs {
				fmt.Printf("  %s = %v\n", k, v)
			}
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "rm <id>",
		Short: "Remove a resource from state",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			store, err := openStore()
			if err != nil {
				return err
			}
			if err := store.Delete(args[0]); err != nil {
				return err
			}
			fmt.Printf("removed %s from state\n", args[0])
			return nil
		},
	})

	return c
}
