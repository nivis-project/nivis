// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nivis-project/nivis/internal/state"
)

// migrateCmd moves the whole state document between the local file store and the
// backend the configuration declares. Unlike every other state command it
// addresses BOTH backends in one run, so it resolves them itself rather than
// through openStore.
//
// It is the supported way to finish the self-managed-bucket bootstrap: apply once
// with --backend=local to create the bucket, then `state migrate --to-remote` to
// move state into it.
func migrateCmd() *cobra.Command {
	var toRemote, fromRemote, force, yes bool
	c := &cobra.Command{
		Use:   "migrate",
		Short: "Move the whole state document between local state and the declared remote backend",
		Long: "Move the whole state document between the local state file and the remote backend\n" +
			"this configuration declares, in the direction you choose. The document is copied,\n" +
			"verified at the destination, and only then removed from the source, so a failure\n" +
			"never loses state. A destination holding different resources is refused unless\n" +
			"--force is given.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			force = force || yes

			// Exactly one direction: guessing which way to move the state of record
			// is not a decision this command may make for the user.
			if toRemote && fromRemote {
				return fmt.Errorf("pass exactly one direction: --to-remote or --from-remote (both were given)")
			}
			if !toRemote && !fromRemote {
				return fmt.Errorf("pass a direction: --to-remote (local state -> the declared backend) " +
					"or --from-remote (the declared backend -> local state)")
			}
			// --backend=local would contradict the point of this command, which is
			// to address both backends at once.
			if backendOverride != "" {
				if err := validateBackendOverride(); err != nil {
					return err
				}
				return fmt.Errorf("--backend has no meaning for `state migrate`: it addresses the local store " +
					"and the declared backend at once, and the direction flags choose which is the source")
			}

			g, err := configGraph(cmd.Context())
			if err != nil {
				return fmt.Errorf("evaluating the configuration to find the declared backend: %w", err)
			}
			if declared := backendType(g.Backend); declared == "" || declared == backendLocal {
				return fmt.Errorf("this configuration declares no remote backend, so there is nothing to " +
					"migrate to or from; declare a `backend` block first (see docs/REMOTE-STATE.md)")
			}

			remote, err := state.OpenBackend(g.Backend, statePath)
			if err != nil {
				return err
			}
			local, err := state.Open(statePath)
			if err != nil {
				return err
			}

			src, dst := state.Store(local), remote
			srcDesc := backendLocation(nil, statePath)
			dstDesc := backendLocation(g.Backend, statePath)
			if fromRemote {
				src, dst = remote, local
				srcDesc, dstDesc = dstDesc, srcDesc
			}

			// Say what is about to move before anything moves.
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Migrating the state document\n  from %s\n  to   %s\n", srcDesc, dstDesc)

			res, err := state.Migrate(src, dst, state.MigrateOptions{Force: force, Operation: "state migrate"})
			if err != nil {
				return err
			}
			if res.Resumed {
				fmt.Fprintln(out, "The destination already held this exact document (an interrupted migration); "+
					"completed it by removing the source.")
			}
			fmt.Fprintf(out, "Moved %d resource(s) to %s; removed the source document at %s.\n",
				res.Resources, dstDesc, srcDesc)
			return nil
		},
	}
	c.Flags().BoolVar(&toRemote, "to-remote", false, "move local state into the backend the configuration declares")
	c.Flags().BoolVar(&fromRemote, "from-remote", false, "move state from the declared backend into the local state file")
	c.Flags().BoolVar(&force, "force", false, "overwrite a destination that holds different state")
	c.Flags().BoolVar(&yes, "yes", false, "alias for --force")
	return c
}
