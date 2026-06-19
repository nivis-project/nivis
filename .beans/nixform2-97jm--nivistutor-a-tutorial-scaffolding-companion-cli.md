---
# nixform2-97jm
title: 'nivistutor: a tutorial-scaffolding companion CLI'
status: in-progress
type: epic
priority: normal
tags:
    - discovered
    - roadmap
created_at: 2026-06-19T12:32:17Z
updated_at: 2026-06-19T12:44:15Z
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
