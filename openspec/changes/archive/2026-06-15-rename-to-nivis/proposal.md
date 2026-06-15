# Proposal: rename-to-nivis

## Why
"Terrae Nivis" is too much; **Nivis** (the meaningful half — *snow*, and it
belongs to Nix) stands on its own (beans-9pf9). Rename the project to **Nivis**,
shorten the CLI to `nivis`, and adopt the payoff line **"All your base belongs to
Nix."** This is a full rename — brand, CLI, Go module, flake interface, repo, and
docs URL.

## What changes
- **Brand & payoff.** Display name `Terrae Nivis` → **Nivis** everywhere
  (README, docs, the mdBook site, the CLI splash, `docs/BRAND.md`). Tagline stays
  "Infrastructure as Nix Code"; add the payoff **"All your base belongs to Nix."**
  Keep the "(formerly nixform / Terrae Nivis)" lineage note once.
- **CLI.** `cmd/tn` → `cmd/nivis` (binary `nivis`). The codegen helper `tn-gen`
  is **folded into the main CLI as `nivis gen`** (one binary, a cobra subcommand)
  rather than a second binary.
- **Go module.** `module github.com/wearetechnative/terrae-nivis` →
  `…/nivis`; every internal import path updated.
- **Flake interface.** The default `--attr` becomes `nivis.plan`; flake outputs
  `terraeNivis.*` → `nivis.*`; `apps`/`packages` expose `nivis` (and `gen`). The
  in-repo examples and the tutorial boilerplate use `outputs.nivis.plan`.
- **Repo & URLs.** Module/repo/Pages references become
  `github.com/wearetechnative/nivis` and `wearetechnative.github.io/nivis`
  (goreleaser owner/name, the release + docs workflows, the flake input URL the
  tutorial tells users to copy, og:image). **The GitHub repo rename itself is
  done by the user**; this change makes the repo internally consistent with the
  new name, and the remote/push is updated after the rename.

## Non-goals
- A new logo/emblem or colour change — same visual brand, just the shorter name.
- Backwards-compat aliases (`tn` shim, `terraeNivis.plan` alias) — the bean says
  users can alias themselves; we make a clean break.
- Rewriting archived OpenSpec changes or the `.beans/` audit trail (historical).
- Cutting a release as part of this (release machinery already exists; a tag is
  separate).

## Impact
- Wide but mechanical: ~700 occurrences across code, docs, flake, CI. Load-bearing
  changes: `go.mod` module path + all imports; `cmd/` layout; `flake.nix`
  attrs/apps; goreleaser + workflows; the docs-site URLs/og:image and the
  docs-SSOT check fingerprints.
- Verification: `go build`/`go test` pass with the new module path; `nix run
  .#nivis -- --version` and `nivis gen` work; `nix eval .#nivis.plan` works and the
  library stays pure; `mdbook build` + docs-SSOT pass; `goreleaser check` passes.
- The **commit is not pushed** until the user renames the GitHub repo; then the
  origin remote is updated and pushed.
- Closes beans-9pf9.
