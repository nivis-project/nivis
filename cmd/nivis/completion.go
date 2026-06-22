// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"

	"github.com/spf13/cobra"
)

// stateIDs is a Cobra completion function suggesting the resource ids present in
// the local state file: used for the `state show`/`state rm` argument and the
// `--target` flag. On any error (no state file yet, unreadable) it suggests
// nothing and disables filename fallback, so an empty/missing store does not
// complete to files in the directory.
func stateIDs(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	// only complete the first positional argument (a single id)
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	store, err := openStore(context.Background())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	items, err := store.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	ids := make([]string, 0, len(items))
	for _, rs := range items {
		ids = append(ids, rs.ID)
	}
	return ids, cobra.ShellCompDirectiveNoFileComp
}
