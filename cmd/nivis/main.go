// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Command nivis is the Nivis executor CLI: plan/apply/destroy/refresh/state/gen
// over a Nix flake that exposes nivis.plan. Pure orchestration; providers are
// spawned from the IR's provider source paths.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/nivis-project/nivis/internal/destroy"
	"github.com/nivis-project/nivis/internal/ir"
	"github.com/nivis-project/nivis/internal/ledger"
	"github.com/nivis-project/nivis/internal/phase"
	"github.com/nivis-project/nivis/internal/plan"
	"github.com/nivis-project/nivis/internal/plugin"
	"github.com/nivis-project/nivis/internal/refresh"
	"github.com/nivis-project/nivis/internal/registry"
	"github.com/nivis-project/nivis/internal/state"
	"github.com/nivis-project/nivis/internal/vars"
)

var (
	flakeRef  string
	statePath string
	target    string
	attr      string
	doRefresh bool
	doBuild   bool
	varFlags  []string
	varFiles  []string
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
	root.PersistentFlags().StringArrayVar(&varFlags, "var", nil, "set a config variable: --var name=value (repeatable; highest precedence)")
	root.PersistentFlags().StringArrayVar(&varFiles, "var-file", nil, "read config variables from a JSON file (repeatable; later files win)")
	root.Flags().BoolVar(&showVersion, "version", false, "print version and exit")

	// --target completes to the resource ids in state.
	_ = root.RegisterFlagCompletionFunc("target", stateIDs)

	root.AddCommand(planCmd(), applyCmd(), destroyCmd(), refreshCmd(), stateCmd(), outputCmd(), genCmd(), forceUnlockCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// evaluator builds a real Nix evaluator from the flags.
func evaluator() phase.NixEval {
	return phase.NixEval{FlakeRef: flakeRef, Attr: attr, WorkDir: ""}
}

// newLedger builds a phase-0 ledger with the resolved configuration variables
// attached (constant across phases). Returns an error for a malformed --var or
// an unreadable/non-object --var-file.
func newLedger() (*ledger.Ledger, error) {
	resolved, err := vars.Resolve(os.Environ(), varFiles, varFlags)
	if err != nil {
		return nil, err
	}
	l := ledger.New()
	l.Vars = resolved
	return l, nil
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
	l, err := newLedger()
	if err != nil {
		return nil, err
	}
	irJSON, err := evaluator().Eval(ctx, l)
	if err != nil {
		return nil, err
	}
	return ir.IngestIR(irJSON)
}

// openStore opens the state backend the config selects. It tries to evaluate the
// config once (phase 0) to read the optional IR `backend` block: when present
// (e.g. an s3 backend) the store is opened from it; absent, the local file store
// at --state is used (today's default). The location comes from `backend`;
// credentials never do (the AWS chain).
//
// The eval is only needed to DISCOVER a remote backend. If it fails (no flake in
// the working dir, an eval error), openStore falls back to the local store rather
// than failing: state subcommands and completion that operate on the document
// without a config (e.g. `state pull` in a bare dir) still work, and the local
// store is the correct default when no backend is declared. Commands that must
// evaluate the config (plan/apply/destroy/refresh) surface that eval error
// themselves; here a failed eval simply means "no remote backend discovered".
func openStore(ctx context.Context) (state.Store, error) {
	g, err := phase0Graph(ctx)
	if err != nil {
		// Could not evaluate the config to learn the backend: default to local.
		return state.Open(statePath)
	}
	return state.OpenBackend(g.Backend, statePath)
}

// withStateLock runs fn while holding the backend's advisory state lock, for a
// mutating operation (apply/destroy). If the store does not support locking (the
// local file store), it just runs fn (unlocked, as today). On a held lock the
// acquire fails before fn runs (naming the holder). The lock is released after fn,
// including on failure, so a failed run never leaves the state locked.
func withStateLock(w io.Writer, store state.Store, operation string, fn func() error) error {
	lk, ok := store.(state.Locker)
	if !ok {
		return fn()
	}
	id, err := lk.Lock(state.NewLockInfo(operation))
	if err != nil {
		return err
	}
	fmt.Fprintln(w, "Acquired state lock.")
	defer func() {
		if uerr := lk.Unlock(id); uerr != nil {
			fmt.Fprintln(w, "warning: releasing state lock:", uerr)
		} else {
			fmt.Fprintln(w, "Released state lock.")
		}
	}()
	return fn()
}

func planCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "plan",
		Short: "Evaluate the configuration and show what would be applied",
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Plan each resource against its prior state via the provider, so an
			// unchanged resource reports no change (not a blanket "~").
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
			d := &phase.Driver{Eval: evaluator(), Manager: mgr, Store: store, Ledger: l, NoRefresh: !doRefresh, NoBuild: !doBuild}

			items, err := d.PlanReport(cmd.Context())
			if err != nil {
				return err
			}
			out := newOutput(cmd.OutOrStdout())
			changes := 0
			for _, it := range items {
				switch it.Op {
				case plan.OpNoop:
					out.noop(it.ID, it.Type)
				case plan.OpUpdate:
					out.update(it.ID, it.Type)
					changes++
				case plan.OpReplace:
					out.replace(it.ID, it.Type)
					changes++
				default:
					out.create(it.ID, it.Type)
					changes++
				}
			}
			if changes == 0 {
				out.printf("\nNo changes. %d resource(s) up to date.\n", len(items))
			} else {
				out.printf("\n%d change(s) across %d resource(s) (+ create, ~ update, -/+ replace, = no change). Run `nivis apply`.\n", changes, len(items))
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
			d := &phase.Driver{
				Eval:       evaluator(),
				Manager:    mgr,
				Store:      store,
				Ledger:     l,
				LedgerPath: statePath + ".ledger",
				NoRefresh:  !doRefresh,
				NoBuild:    !doBuild,
			}
			// Hold the state lock for the whole apply (no-op on an unlockable store).
			var res *phase.Result
			if err := withStateLock(cmd.OutOrStdout(), store, "apply", func() error {
				r, runErr := d.Run(cmd.Context())
				res = r
				return runErr
			}); err != nil {
				return err
			}
			out := newOutput(cmd.OutOrStdout())
			out.printf("Applied %d resource(s) across %d phase(s):\n\n", len(res.Applied), res.AppliedPhases)
			for i, group := range res.Phases {
				out.phaseHeading(i + 1)
				for _, n := range group {
					switch {
					case n.IsData:
						out.read(n.ID, "") // a datasource READ, not a create
					case n.Op == plan.OpNoop:
						out.noop(n.ID, "")
					case n.Op == plan.OpUpdate:
						out.update(n.ID, "")
					case n.Op == plan.OpReplace:
						out.replace(n.ID, "")
					default: // OpCreate
						out.create(n.ID, "")
					}
				}
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
			store, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			mgr := newManager()
			defer mgr.Close()
			// Hold the state lock for the destroy (no-op on an unlockable store).
			var res *destroy.Result
			if err := withStateLock(cmd.OutOrStdout(), store, "destroy", func() error {
				r, runErr := destroy.Run(cmd.Context(), g, mgr, store, destroy.Options{Target: target})
				res = r
				return runErr
			}); err != nil {
				return err
			}
			out := newOutput(cmd.OutOrStdout())
			out.printf("Destroyed %d resource(s):\n", len(res.Destroyed))
			for _, id := range res.Destroyed {
				out.destroy(id, "")
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
			store, err := openStore(cmd.Context())
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			items, err := store.List()
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			if len(items) == 0 {
				fmt.Fprintln(w, "No resources in state.")
				return nil
			}
			for _, rs := range items {
				fmt.Fprintln(w, rs.ID)
			}
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:               "show <id>",
		Short:             "Show a resource's stored attributes",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: stateIDs,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openStore(cmd.Context())
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
		Use:               "rm <id>",
		Short:             "Remove a resource from state",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: stateIDs,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			if _, ok, err := store.Get(args[0]); err != nil {
				return err
			} else if !ok {
				return fmt.Errorf("%q is not in state (nothing to remove)", args[0])
			}
			if err := store.Delete(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s from state\n", args[0])
			return nil
		},
	})

	c.AddCommand(pullCmd(), pushCmd())

	return c
}
