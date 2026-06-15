// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Command nivis is the Nivis executor CLI: plan/apply/destroy/refresh/state/gen
// over a Nix flake that exposes nivis.plan. Pure orchestration; providers are
// spawned from the IR's provider source paths.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/wearetechnative/nivis/internal/destroy"
	"github.com/wearetechnative/nivis/internal/ir"
	"github.com/wearetechnative/nivis/internal/ledger"
	"github.com/wearetechnative/nivis/internal/phase"
	"github.com/wearetechnative/nivis/internal/plan"
	"github.com/wearetechnative/nivis/internal/plugin"
	"github.com/wearetechnative/nivis/internal/refresh"
	"github.com/wearetechnative/nivis/internal/registry"
	"github.com/wearetechnative/nivis/internal/state"
)

var (
	flakeRef  string
	statePath string
	target    string
	attr      string
	doRefresh bool
	doBuild   bool
)

func main() {
	var showVersion bool
	root := &cobra.Command{
		Use:   "nivis",
		Short: "Nivis — Infrastructure as Nix Code",
		Long: "Nivis — Infrastructure as Nix Code. All your base belongs to Nix.\n\n" +
			"A Nix-native infrastructure tool where Terraform/OpenTofu provider\n" +
			"resources are first-class Nix values. (Formerly nixform, then Terrae Nivis.)",
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
				fmt.Fprintln(cmd.OutOrStdout(), "nivis "+version)
				return
			}
			splash(cmd.OutOrStdout())
		},
	}
	root.PersistentFlags().StringVar(&flakeRef, "flake", ".", "flake reference exposing nivis.plan")
	root.PersistentFlags().StringVar(&statePath, "state", "./nivis.state.json", "path to the local state file")
	root.PersistentFlags().StringVar(&attr, "attr", "nivis.plan", "flake attribute to evaluate")
	root.PersistentFlags().StringVar(&target, "target", "", "restrict the operation to a single resource id")
	root.PersistentFlags().BoolVar(&doRefresh, "refresh", true, "read real provider state before planning (false = plan against stored state)")
	root.PersistentFlags().BoolVar(&doBuild, "build", true, "realise Nix build outputs (drv leaves) referenced by resources (false = assume already built)")
	root.Flags().BoolVar(&showVersion, "version", false, "print version and exit")

	root.AddCommand(planCmd(), applyCmd(), destroyCmd(), refreshCmd(), stateCmd(), genCmd())

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
			// Plan each resource against its prior state via the provider, so an
			// unchanged resource reports no change (not a blanket "~").
			store, err := openStore()
			if err != nil {
				return err
			}
			mgr := newManager()
			defer mgr.Close()
			d := &phase.Driver{Eval: evaluator(), Manager: mgr, Store: store, Ledger: ledger.New(), NoRefresh: !doRefresh, NoBuild: !doBuild}

			items, err := d.PlanReport(cmd.Context())
			if err != nil {
				return err
			}
			changes := 0
			for _, it := range items {
				marker := "+"
				switch it.Op {
				case plan.OpNoop:
					marker = "="
				case plan.OpUpdate:
					marker = "~"
				case plan.OpReplace:
					marker = "-/+"
				}
				if it.Op != plan.OpNoop {
					changes++
				}
				fmt.Printf("%s %s (%s)\n", marker, it.ID, it.Type)
			}
			if changes == 0 {
				fmt.Printf("\nNo changes. %d resource(s) up to date.\n", len(items))
			} else {
				fmt.Printf("\n%d change(s) across %d resource(s) (+ create, ~ update, -/+ replace, = no change). Run `nivis apply`.\n", changes, len(items))
			}
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
				NoRefresh:  !doRefresh,
				NoBuild:    !doBuild,
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
