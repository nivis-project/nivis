# Releasing Nivis

The version is a single source of truth: the top-level **`VERSION`** file. Both
the Nix flake and the Go binary derive from it (the flake injects it via
`-ldflags -X main.version`; a plain `go build` with no ldflags reports `dev`).

## Cut a release

```sh
scripts/release.sh patch   # 0.2.0 -> 0.2.1   (bug fixes)
scripts/release.sh minor   # 0.2.0 -> 0.3.0   (features, backwards-compatible)
scripts/release.sh major   # 0.2.0 -> 1.0.0   (breaking)
scripts/release.sh minor --dry-run   # show the plan, change nothing
```

`release.sh`:
1. bumps `VERSION`,
2. rolls `CHANGELOG.md`'s `## [Unreleased]` into a dated `## [<version>]` section
   (so write your changes under **Unreleased** as you go),
3. commits (`jj` if present, else `git`),
4. creates and **pushes** a `v<version>` git tag.

## What the tag triggers

Pushing the tag runs **`.github/workflows/release.yml`**, which uses
[goreleaser](https://goreleaser.com) to:
- cross-build `nivis` and `nivis gen` for linux/darwin × amd64/arm64 (version from the
  tag),
- write `checksums.txt` and changelog-derived release notes,
- create the **GitHub release** with the archives attached.

`v0.x` tags are marked pre-release automatically.

## Keeping the changelog current (the changelog gate)

The changelog is kept current as you go, tied to OpenSpec archival. Every
`proposal.md` carries a `Changelog:` line:

```
Changelog: Added datasources (nivis.mkData): read existing infra and feed it in.
Changelog: none - internal refactor, no user-facing surface.
```

`tests/check-changelog.sh` (run inside `tests/check-docs-ssot.sh`) enforces that
every archived change has the line, and that a non-`none` entry actually appears
in `CHANGELOG.md`'s `## [Unreleased]` section (matched formatting- and
wrapping-tolerantly). So a user-facing change cannot be archived without its
changelog entry being present. Changes archived before the convention are exempt.

## Before tagging

Write your changes under `## [Unreleased]` in `CHANGELOG.md` (the changelog gate
keeps this honest). Validate the release config locally if you've touched it:

```sh
goreleaser check                       # validate .goreleaser.yaml
goreleaser release --snapshot --clean  # dry build (no publish) -> ./dist
```

## Closing a milestone

A version tag is the per-release artifact; a **milestone** is a coherent batch of
capability (e.g. "Road to v1"). When a milestone's epics are all done, it gets
**release notes** that show what a user can now do, drawn from the tutorials'
verified examples, not just a changelog.

1. Mark the milestone **completed** in beans (after its child epics are done).
2. Feature the runnable examples: in the relevant tutorial(s), wrap the blocks
   worth highlighting in `<!-- release-note: <title> -->` ... `<!-- /release-note -->`.
   These are pulled verbatim, so they stay verified where they live.
3. Generate the notes:

   ```sh
   scripts/milestone-notes.sh <milestone-id>   # writes docs/releases/<slug>.md
   ```

   The notes assemble: the milestone goal, **Highlights** (the marked tutorial
   blocks), **What shipped** (the completed child epics), and the `CHANGELOG`
   `[Unreleased]` section. The file is generated, do not hand-edit it; rerun the
   generator.
4. Commit `docs/releases/<slug>.md`.

`tests/check-milestone-notes.sh` (run inside `tests/check-docs-ssot.sh`) is a
golden gate: every completed milestone must have notes that **regenerate
identically**, so missing or stale notes fail the check. Milestones that closed
before the gate existed (the PoC) are exempt.

## Conventions

- Semantic versioning. `0.1.x` was the PoC; `0.2.x` onward is active development.
- The `VERSION` file holds a bare semver (e.g. `0.2.0`); tags are `v`-prefixed
  (`v0.2.0`).
