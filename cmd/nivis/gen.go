// Copyright 2026 WeareTechnative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/wearetechnative/nivis/internal/gen"
	"github.com/wearetechnative/nivis/internal/plugin"
)

// genCmd is `nivis gen`: generate typed Nix constructors from a provider's
// schema. Spawn a provider, fetch its schema, emit <out>/<provider>/<type>.nix.
// The path to "all providers with zero per-provider work" (DESIGN D2).
func genCmd() *cobra.Command {
	var (
		providerPath string
		identity     string
		outDir       string
	)
	cmd := &cobra.Command{
		Use:   "gen",
		Short: "Generate typed Nix constructors from a provider schema",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if providerPath == "" {
				return fmt.Errorf("--provider is required")
			}
			if identity == "" {
				identity = providerIdentity(providerPath)
			}
			mgr := plugin.NewManager()
			defer mgr.Close()

			// Codegen only fetches the schema; configure with an empty config
			// (real providers accept an all-null configure and the schema RPC
			// works regardless).
			client, err := mgr.Client(identity, providerPath, map[string]interface{}{})
			if err != nil {
				return err
			}
			resources, err := gen.Fetch(context.Background(), client)
			if err != nil {
				return err
			}
			dir := filepath.Join(outDir, identity)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			for _, r := range resources {
				out := filepath.Join(dir, r.Type+".nix")
				if err := os.WriteFile(out, []byte(gen.Emit(identity, r)), 0o644); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "generated %d resource constructor(s) for provider %q\n", len(resources), identity)
			return nil
		},
	}
	cmd.Flags().StringVar(&providerPath, "provider", "", "path to the provider binary")
	cmd.Flags().StringVar(&identity, "identity", "", "provider identity (default: binary basename)")
	cmd.Flags().StringVar(&outDir, "out", "./generated", "output directory")
	return cmd
}

// providerIdentity strips the directory and a leading "provider-" prefix so a
// binary named "provider-alpha" yields identity "alpha".
func providerIdentity(p string) string {
	b := filepath.Base(p)
	const pfx = "provider-"
	if len(b) > len(pfx) && b[:len(pfx)] == pfx {
		return b[len(pfx):]
	}
	return b
}
