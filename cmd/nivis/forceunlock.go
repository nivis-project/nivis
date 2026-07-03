// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/nivis-project/nivis/internal/state"
)

// forceUnlockCmd clears a stuck state lock left by a crashed run (the escape
// hatch). It confirms first in an interactive session and accepts --force/--yes
// for non-interactive use. A backend that does not support locking (the local file
// store) is reported as having no lock to clear, rather than crashing.
func forceUnlockCmd() *cobra.Command {
	var force, yes bool
	c := &cobra.Command{
		Use:   "force-unlock",
		Short: "Remove a stuck state lock (after a crashed run)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			force = force || yes
			store, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			lk, ok := store.(state.Locker)
			if !ok {
				return fmt.Errorf("the configured state backend does not use locking (nothing to force-unlock)")
			}

			out := cmd.OutOrStdout()
			if !force {
				interactive := false
				if f, ok := cmd.InOrStdin().(*os.File); ok {
					interactive = term.IsTerminal(int(f.Fd()))
				}
				if !interactive {
					return fmt.Errorf("force-unlock removes the state lock; rerun with --force to confirm (required for non-interactive input)")
				}
				fmt.Fprint(out, "This removes the state lock (only do this if no other run is active).\nProceed? [y/N] ")
				if !confirmed(cmd.InOrStdin()) {
					fmt.Fprintln(out, "Aborted; lock unchanged.")
					return nil
				}
			}

			if err := lk.ForceUnlock(); err != nil {
				return err
			}
			fmt.Fprintln(out, "Removed the state lock.")
			return nil
		},
	}
	c.Flags().BoolVar(&force, "force", false, "remove the lock without confirmation (required for non-interactive input)")
	c.Flags().BoolVar(&yes, "yes", false, "alias for --force")
	return c
}
