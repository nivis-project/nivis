// Command nixform-gen generates typed Nix constructors from a provider's schema.
// Spawn a provider, fetch its schema, emit <out>/<provider>/<type>.nix. The path
// to "all providers with zero per-provider work" (DESIGN D2).
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/wearetechnative/nixform/internal/gen"
	"github.com/wearetechnative/nixform/internal/plugin"
)

func main() {
	var (
		providerPath string
		identity     string
		outDir       string
	)
	root := &cobra.Command{
		Use:   "nixform-gen",
		Short: "Generate typed Nix constructors from a provider schema",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if providerPath == "" {
				return fmt.Errorf("--provider is required")
			}
			if identity == "" {
				identity = filepathBase(providerPath)
			}
			mgr := plugin.NewManager()
			defer mgr.Close()

			client, err := mgr.Client(identity, providerPath)
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
				fmt.Printf("wrote %s\n", out)
			}
			fmt.Printf("generated %d resource constructor(s) for provider %q\n", len(resources), identity)
			return nil
		},
	}
	root.Flags().StringVar(&providerPath, "provider", "", "path to the provider binary")
	root.Flags().StringVar(&identity, "identity", "", "provider identity (default: binary basename)")
	root.Flags().StringVar(&outDir, "out", "./generated", "output directory")

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// filepathBase strips the directory and a leading "provider-" prefix so a binary
// named "provider-alpha" yields identity "alpha".
func filepathBase(p string) string {
	b := filepath.Base(p)
	const pfx = "provider-"
	if len(b) > len(pfx) && b[:len(pfx)] == pfx {
		return b[len(pfx):]
	}
	return b
}
