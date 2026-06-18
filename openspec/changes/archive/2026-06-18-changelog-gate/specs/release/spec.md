# Spec delta: release

## ADDED Requirements

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
