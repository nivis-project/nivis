# release Specification

## Purpose
TBD - created by archiving change release-management. Update Purpose after archive.
## Requirements
### Requirement: A single source of truth for the version
The repository SHALL keep the application version in **one** canonical place — a
top-level `VERSION` file containing a bare semver — from which both the Nix flake
and the Go binary derive their version. `flake.nix` SHALL read `VERSION` for the
package version and inject it into the binary (`-ldflags -X main.version`); a
plain `go build` SHALL also report the canonical version (the Go default reads the
embedded `VERSION`). `tn --version` and the splash SHALL show the same value in
all build paths.

#### Scenario: every build path reports the canonical version
- GIVEN `VERSION` contains `0.2.0`
- WHEN `tn --version` is run from a `nix run .#tn` build and from a plain `go build`
- THEN both print `0.2.0` (no hardcoded or divergent version).

### Requirement: Changelog and a semver bump/release script
The repository SHALL include a `CHANGELOG.md` (Keep-a-Changelog format) and a
`scripts/release.sh` accepting `patch`, `minor`, or `major` that bumps `VERSION`
by that increment, rolls the changelog's `Unreleased` section into a dated version
section, commits, and creates and pushes a `v<version>` git tag. It SHALL be
jj/git compatible and SHALL support a `--dry-run` that reports the planned next
version and changes without writing.

#### Scenario: a minor bump
- GIVEN `VERSION` is `0.2.0`
- WHEN `scripts/release.sh minor --dry-run` is run
- THEN it reports the next version `0.3.0` and the changelog/tag it would write, without modifying the working tree.

### Requirement: Tag-triggered GitHub release
A GitHub Actions workflow SHALL, on a pushed tag matching `v*`, build the `tn` and
`tn-gen` binaries for the common platforms (linux/darwin × amd64/arm64) with the
version taken from the tag, and create a GitHub release with the binaries,
checksums, and changelog-derived notes (via goreleaser). The goreleaser
configuration SHALL be valid.

#### Scenario: goreleaser config is valid and builds
- WHEN `goreleaser check` and a snapshot build are run
- THEN the config validates and produces `tn` and `tn-gen` binaries for the configured platforms.

