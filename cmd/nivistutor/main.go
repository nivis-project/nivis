// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Command nivistutor scaffolds a Nivis tutorial's starter files (a working
// flake.nix, the config, and a README) into the user's own directory, so they
// learn by reading, editing, and running the config with plain `nivis`. It does
// NOT run nivis itself. Distributed as the flake app `#tutor`. The starters are
// embedded (go:embed), so it works offline and writes files matching its build.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// version is the displayed version and the nivis ref scaffolded flakes pin to.
// The single source of truth is the top-level VERSION file; release builds (the
// Nix flake / goreleaser) inject it via `-ldflags -X main.version=<v>`. A plain
// `go build` reports "dev".
var version = "dev"

// nivisRef is the github flake ref a scaffolded starter pins its nivis input to.
// For a release build (version like "0.4.3") it is the tag; for a dev build it
// floats on the default branch so a local checkout still resolves.
func nivisRef() string {
	if version == "dev" || version == "" {
		return "github:wearetechnative/nivis"
	}
	return "github:wearetechnative/nivis/v" + version
}

func main() {
	var (
		listOnly    bool
		tutorialArg string
		dirArg      string
		force       bool
		showVersion bool
	)
	cmd := &cobra.Command{
		Use:   "nivistutor",
		Short: "Scaffold a Nivis tutorial into your directory",
		Long: "nivistutor scaffolds a Nivis tutorial's starter files (flake.nix, config,\n" +
			"and README) into your own directory, so you read, edit, and run them with\n" +
			"plain `nivis`. It does not run nivis for you.",
		SilenceErrors: true,
		PersistentPreRun: func(c *cobra.Command, _ []string) {
			c.SilenceUsage = true
		},
		RunE: func(c *cobra.Command, _ []string) error {
			if showVersion {
				fmt.Fprintln(c.OutOrStdout(), "nivistutor "+version)
				return nil
			}
			if listOnly {
				return runList(c.OutOrStdout())
			}
			// Non-interactive when a tutorial is named; otherwise the prompted flow.
			if tutorialArg != "" {
				return runScaffold(c.OutOrStdout(), tutorialArg, dirArg, force)
			}
			return runInteractive(c.InOrStdin(), c.OutOrStdout(), dirArg, force)
		},
	}
	cmd.Flags().BoolVar(&listOnly, "list", false, "list the available tutorials and exit")
	cmd.Flags().StringVar(&tutorialArg, "tutorial", "", "tutorial to scaffold (non-interactive); see --list")
	cmd.Flags().StringVar(&dirArg, "dir", "", "target directory (default: a new <tutorial>/ subdir, or '.' for current)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing files")
	cmd.Flags().BoolVar(&showVersion, "version", false, "print version and exit")

	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// runList prints the available tutorials.
func runList(w io.Writer) error {
	ts, err := listTutorials()
	if err != nil {
		return err
	}
	for _, t := range ts {
		if t.Summary != "" {
			fmt.Fprintf(w, "%s\t%s\n", t.Name, t.Summary)
		} else {
			fmt.Fprintln(w, t.Name)
		}
	}
	return nil
}

// runScaffold writes one tutorial non-interactively and prints the next steps.
// dir defaults to a new subdirectory named after the tutorial.
func runScaffold(w io.Writer, name, dir string, force bool) error {
	t, ok := findTutorial(name)
	if !ok {
		return fmt.Errorf("unknown tutorial %q (see `nivistutor --list`)", name)
	}
	if dir == "" {
		dir = name
	}
	written, err := scaffold(name, scaffoldOptions{Dir: dir, Force: force, NivisRef: nivisRef()})
	if err != nil {
		return err
	}
	printDone(w, t, dir, written)
	return nil
}

// runInteractive is the prompted flow: greet, list, pick, choose target, write,
// and print next steps. It never runs nivis.
func runInteractive(in io.Reader, w io.Writer, dirOverride string, force bool) error {
	greet(w)
	ts, err := listTutorials()
	if err != nil {
		return err
	}
	if len(ts) == 0 {
		return fmt.Errorf("no tutorials are embedded in this build")
	}
	r := bufio.NewReader(in)

	// Pick a tutorial.
	fmt.Fprintln(w, "Available tutorials:")
	for i, t := range ts {
		label := t.Name
		if t.Summary != "" {
			label = fmt.Sprintf("%s — %s", t.Name, t.Summary)
		}
		fmt.Fprintf(w, "  %d. %s\n", i+1, label)
	}
	fmt.Fprintf(w, "\nWhich tutorial? [1-%d] (default 1): ", len(ts))
	pick := readLine(r)
	idx := 0
	if pick != "" {
		n, perr := strconv.Atoi(pick)
		if perr != nil || n < 1 || n > len(ts) {
			return fmt.Errorf("not a choice between 1 and %d: %q", len(ts), pick)
		}
		idx = n - 1
	}
	chosen := ts[idx]

	// Choose target: new subdir or current directory.
	dir := dirOverride
	if dir == "" {
		fmt.Fprintf(w, "\nScaffold into:\n  a. a new subdirectory (%s/)\n  b. the current directory\n", chosen.Name)
		fmt.Fprint(w, "Choice? [a/b] (default a): ")
		switch strings.ToLower(readLine(r)) {
		case "", "a":
			dir = chosen.Name
		case "b":
			dir = "."
		default:
			return fmt.Errorf("please answer a or b")
		}
	}

	written, err := scaffold(chosen.Name, scaffoldOptions{Dir: dir, Force: force, NivisRef: nivisRef()})
	if err != nil {
		return err
	}
	printDone(w, chosen, dir, written)
	return nil
}

// printDone reports the written files and the exact next steps. It does not run
// nivis; the user does, after reading the config.
func printDone(w io.Writer, t tutorial, dir string, written []string) {
	fmt.Fprintf(w, "\nTutorial config files have been created in %s:\n", dir)
	for _, f := range written {
		fmt.Fprintf(w, "  %s\n", filepath.ToSlash(filepath.Join(dir, f)))
	}
	fmt.Fprintln(w, "\nYou can now continue learning Nivis. Next steps:")
	if dir != "." {
		fmt.Fprintf(w, "  cd %s\n", dir)
	}
	fmt.Fprintln(w, "  nivis plan")
	fmt.Fprintln(w, "  nivis apply")
	fmt.Fprintln(w, "  nivis output")
	fmt.Fprintln(w, "\nRead README.md in that directory for the full walkthrough.")
}

func readLine(r *bufio.Reader) string {
	s, _ := r.ReadString('\n')
	return strings.TrimSpace(s)
}
