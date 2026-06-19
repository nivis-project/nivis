# Proposal: nivistutor

## Why
Running a tutorial in a clean sandbox is awkward today: the tutorial config lives
in the repo flake's `nivis.tutorial` attribute, so a user without a checkout must
pass `--flake github:.../<tag> --attr nivis.tutorial`. And a black-box "run it for
me" command teaches nothing. `nivistutor` instead **scaffolds** a tutorial's
files into the user's own directory, so they read, edit, and run the config with
plain `nivis` (learn by doing). It is the first piece of a companion tool that
will expand as tutorials accrue (beans-97jm).

## What changes
- **A `nivistutor` CLI**, distributed as the flake app `#tutor`:
  ```sh
  nix shell github:wearetechnative/nivis#nivis github:wearetechnative/nivis#tutor
  nivistutor
  ```
  It greets the user, lists the available tutorials, lets them pick one, asks
  whether to scaffold into a **new subdirectory** or the **current directory**,
  writes that tutorial's starter files, and prints the next steps (the exact
  `nivis …` commands). It does **not** run `nivis` for the user.
- **Tutorials become self-contained starter directories.** Each lives at
  `nix/example/tutorial-<name>/` with its own working `flake.nix` (a `nivis`
  input + `nivis.plan`), the config, and a `README.md` of next steps, so once
  scaffolded the user runs `nivis plan/apply/output` with no `--flake`/`--attr`.
  The starters are **embedded in the binary** (`go:embed`), so `nivistutor` works
  offline and always writes the files matching its own build (version-locked).
- **Two starters to begin with** (so the menu has real choices):
  - **`getting-started`** — the always-current "learn Nivis from scratch"
    tutorial (the headline two-provider round trip, from `nix/example/default.nix`
    + the `GETTING-STARTED.md` walkthrough). This is the entry-point tutorial and
    will grow/change across releases.
  - **`features-0.4`** — the per-release "what's new in 0.4" tutorial (the current
    `tutorial.nix`: variables, datasource, outputs, phased apply).
- The offline starters use the in-repo fake providers (bare-name `source`), so
  their README tells the user to enter `nix shell …#nivis …#tutor` (which also
  carries the fakes) before running. (The `#fake-providers` package stays; `#tutor`
  re-exposes the providers on PATH so one shell has everything.)

## Decisions (settled with the maintainer)
- **Scaffold files, do not run** (learning by doing, not a black-box e2e).
- **Embed the starters** (`go:embed`), not fetch at runtime (offline,
  version-locked to the build).
- **Self-contained starters** with a working `flake.nix` each (release tutorials
  focus on new features and assume Nivis basics, so they ship a ready flake).
- **Menu from the start**, because there are two real tutorials (getting-started +
  features-0.4).
- Binary `nivistutor`, flake app `#tutor`.

## Non-goals
- Running/grading the tutorial for the user.
- Fetching tutorials over the network (embedded only).
- Renaming `#fake-providers` (kept; `#tutor` provides the providers on PATH too).
- A full TUI; a simple prompted CLI is enough for v1.

## Impact
- New: `cmd/nivistutor` (the CLI, with `go:embed` of the starter dirs), a `#tutor`
  flake app + a `nivistutor` package.
- New: `nix/example/tutorial-getting-started/` and
  `nix/example/tutorial-features-0.4/`, each a self-contained starter
  (flake.nix + config + README). The repo flake's `nivis.tutorial` (and the
  feature tutorial doc / milestone notes) point at the features-0.4 starter's
  config so the golden gate and the docs include stay valid.
- Docs: the feature tutorial + getting-started note the `nivistutor` path for a
  sandbox; a short section on `nivistutor`.
- Tests: a Go test that `nivistutor` writes the expected files for a chosen
  tutorial into a temp dir (non-interactive mode via a flag), and that each
  embedded starter's `flake.nix` is present. The existing e2e still drives the
  configs directly.

Changelog: Added nivistutor, a companion CLI (flake app `#tutor`) that scaffolds a
chosen tutorial's starter files (flake + config + README) into your directory so
you run them with plain `nivis`; ships a getting-started and a features-0.4
tutorial.

Docs impact: new section (a "nivistutor" / sandbox note in the tutorials) plus the
embedded starter READMEs. No standalone new doc; it extends the tutorial docs (per
docs/DOCS-GATE.md).
