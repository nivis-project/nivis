---
# nixform2-97jm
title: 'nivistutor: a tutorial-scaffolding companion CLI'
status: completed
type: epic
priority: normal
tags:
    - discovered
    - roadmap
created_at: 2026-06-19T12:32:17Z
updated_at: 2026-06-19T14:22:38Z
---

A small, EXPANDING companion application that scaffolds Nivis tutorial config files into the user's own directory, so they learn by reading/editing/running the configs with plain `nivis` (NOT a black-box e2e run, which teaches nothing).

## Vision (from the maintainer)
Distributed via the flake as `#tutor` (a flake app). Sandbox UX:
  nix shell github:wearetechnative/nivis#nivis github:wearetechnative/nivis#tutor
  nivistutor
  -> "Welcome to the Nivis Tutorials. <intro>"
     a. Create a new tutorial directory with config files?
     b. Create the files in the current directory?
  -> "tutorial config files have been created. You can now continue learning Nivis."

So nivistutor WRITES the tutorial's config (e.g. the feature tutorial's flake/config) to disk for the user, then the user runs `nivis plan/apply/output ...` themselves. It does not run nivis for them.

## Why this shape
The tutorial config is evaluated as a flake attr (nix eval ...#nivis.tutorial), which is awkward in a clean sandbox (needs --flake <tag> --attr). Scaffolding the files locally gives the user a real, editable project that `nivis` runs with no flags, and it teaches by exposing the config.

## Expanding
It will grow (more tutorials per release, list/pick a tutorial, maybe templates). Treat as its own epic; tasks become OpenSpec changes. Relates to the per-release tutorial naming (tutorial-features-<version>) and the existing #fake-providers package (the scaffolded tutorial still needs the fake providers on PATH).

## Open questions to settle before the first OpenSpec change
- Does it embed the tutorial assets in the binary (go:embed) or fetch from the repo at a ref?
- Naming: binary `nivistutor`, flake app `#tutor` (or `#nivistutor`)?
- Does it also drop a README/next-steps file alongside the config?
- Per-release: which tutorial(s) does a given build offer?



## MVP delivered (2026-06-19, OpenSpec change nivistutor archived as 2026-06-19-nivistutor)

nivistutor ships:
- cmd/nivistutor: cobra CLI with a branded greeting, an interactive flow (greet -> list -> pick -> new-subdir vs current-dir -> write -> print next steps; does NOT run nivis), and a non-interactive mode (--tutorial/--dir/--force/--list). Starters embedded via go:embed (offline, version-locked). No-clobber without --force.
- Self-contained starters at nix/example/tutorial-getting-started/ and nix/example/tutorial-features-0.4/ (each: working flake.nix pinned to the scaffolding build's nivis release via the @NIVIS_REF@ placeholder nivistutor rewrites, config.nix, README.md). The repo flake's nivis.tutorial now points at the features-0.4 starter config (milestone-notes golden gate + docs include stay valid); old nix/example/tutorial.nix removed.
- flake: #tutor package + app bundling nivistutor with the fake providers, so one 'nix shell .#nivis .#tutor' carries everything; 'nix run .#tutor' scaffolds.
- Tests: cmd/nivistutor unit tests (writes expected files + ref substitution, no-clobber atomicity, unknown-tutorial, every embedded starter complete, menu has getting-started + a features tutorial, embedded copies byte-match the canonical nix/example starters). run-nix-tests.sh step 6 validates both starters produce conforming IR.
- Docs: a 'Scaffold a tutorial with nivistutor' section in GETTING-STARTED.md (surfaced on the site) + a sandbox note in TUTORIAL-FEATURES.md.
- Also fixed the changelog gate (tests/check-changelog.sh) to search the whole changelog, not only [Unreleased], so a change archived before its release is not flagged once its entry rolls into a dated section.

gofmt / go build ./... / go test ./... / nix tests / docs-ssot gate all green. Targeting release 0.4.3.

## Expanding (follow-ups, not blocking)

This epic stays open as the tutorial set grows:
- Add a tutorial per release (tutorial-features-<version>); the menu auto-lists embedded starters.
- Richer menu (descriptions/longer blurbs, maybe grouping entry vs per-release).
- Possibly a getting-started-with-AWS starter once the real-provider path is tutorial-ready.
- Consider folding nix/example/tutorial-* and the embedded copy into a single source if the cp-sync (guarded by TestEmbeddedStartersMatchCanonical) becomes a burden.
