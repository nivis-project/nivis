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

### Requirement: Milestone release notes are generated and gated
When a milestone closes, the project SHALL have a generated release-notes document
for it under `docs/releases/`, assembled deterministically from three sources by a
generator (`scripts/milestone-notes.sh <milestone-id>`):
- **Highlights**: blocks marked in the tutorials with
  `<!-- release-note: <title> -->` ... `<!-- /release-note -->`, pulled verbatim
  (the verified, runnable examples).
- **What shipped**: the titles of the milestone's completed child epics, from the
  beans tracker.
- **Changelog**: the current `## [Unreleased]` section of `CHANGELOG.md`.

A gate SHALL enforce that every **completed** milestone has such a document and
that the document **regenerates identically** (running the generator and diffing
against the committed file produces no difference), so stale or missing milestone
notes fail the check. The gate SHALL run as part of the docs checks. Generation
SHALL be deterministic (stable ordering, no timestamps that change per run) so the
golden comparison is reliable.

#### Scenario: a closed milestone has regenerable notes
- GIVEN a milestone marked completed in beans
- WHEN the milestone-notes gate runs
- THEN it finds `docs/releases/<slug>.md`, regenerates it, and the regenerated content matches the committed file.

#### Scenario: missing or stale notes fail the gate
- GIVEN a completed milestone whose release-notes doc is absent or no longer matches what the generator produces
- WHEN the gate runs
- THEN it fails, naming the milestone and pointing at `scripts/milestone-notes.sh`.

#### Scenario: highlights come from marked tutorial blocks
- GIVEN a tutorial with a `release-note`-marked block
- WHEN the notes are generated
- THEN that block appears verbatim under Highlights in the milestone's notes.

### Requirement: Archived changes declare and reflect a changelog entry
Every OpenSpec change SHALL record its changelog status in `proposal.md` as a line
beginning `Changelog:` whose value is either the entry text to add under
`## [Unreleased]` in `CHANGELOG.md`, or `none` (with a short reason) for an
internal or non-user-facing change. A gate SHALL enforce, over archived changes:
- each has a `Changelog:` line;
- for a line whose value is not `none`, a distinctive fingerprint of the declared
  entry appears in the `## [Unreleased]` section of `CHANGELOG.md`.
The gate SHALL run as part of the docs checks. Changes archived before this
convention existed (by archive date) are exempt. The gate does not judge wording;
it ensures the call was made and a declared user-facing entry is present before
release.

#### Scenario: a user-facing archived change is reflected in the changelog
- GIVEN an archived change whose proposal has `Changelog: Added X`
- WHEN the gate runs
- THEN it passes only if the `## [Unreleased]` section of `CHANGELOG.md` contains that entry.

#### Scenario: an internal change is exempt from a changelog entry
- GIVEN an archived change whose proposal has `Changelog: none - <reason>`
- WHEN the gate runs
- THEN it passes with no required changelog entry.

#### Scenario: a missing Changelog line fails
- GIVEN an archived (post-convention) change whose proposal has no `Changelog:` line
- WHEN the gate runs
- THEN it fails, naming the change and pointing at the convention.

