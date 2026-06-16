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

## Before tagging

Write your changes under `## [Unreleased]` in `CHANGELOG.md`. Validate the
release config locally if you've touched it:

```sh
goreleaser check                       # validate .goreleaser.yaml
goreleaser release --snapshot --clean  # dry build (no publish) -> ./dist
```

## Conventions

- Semantic versioning. `0.1.x` was the PoC; `0.2.x` onward is active development.
- The `VERSION` file holds a bare semver (e.g. `0.2.0`); tags are `v`-prefixed
  (`v0.2.0`).
