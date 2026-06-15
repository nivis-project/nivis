// Command nixform is the executor CLI: plan/apply/destroy/refresh/state over a
// Nix flake that exposes nixform.plan. Pure orchestration; providers are spawned
// from the IR's provider source paths.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/wearetechnative/nixform/internal/destroy"
	"github.com/wearetechnative/nixform/internal/ir"
	"github.com/wearetechnative/nixform/internal/ledger"
	"github.com/wearetechnative/nixform/internal/phase"
	"github.com/wearetechnative/nixform/internal/plugin"
	"github.com/wearetechnative/nixform/internal/refresh"
	"github.com/wearetechnative/nixform/internal/state"
)

var (
	flakeRef  string
	statePath string
	target    string
	attr      string
)

func main() {
	root := &cobra.Command{
		Use:   "nixform",
		Short: "Nix-native infra: provider resources as first-class Nix values",
	}
	root.PersistentFlags().StringVar(&flakeRef, "flake", ".", "flake reference exposing nixform.plan")
	root.PersistentFlags().StringVar(&statePath, "state", "./nixform.state.json", "path to the local state file")
	root.PersistentFlags().StringVar(&attr, "attr", "nixform.plan", "flake attribute to evaluate")
	root.PersistentFlags().StringVar(&target, "target", "", "restrict the operation to a single resource id")

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
			fmt.Printf("\n%d resource(s) to resolve across phases. Run `nixform apply`.\n", len(g.Order))
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
			mgr := plugin.NewManager()
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
			mgr := plugin.NewManager()
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
			mgr := plugin.NewManager()
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
