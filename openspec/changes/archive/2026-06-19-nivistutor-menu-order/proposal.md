# Proposal: nivistutor-menu-order

## Why
`nivistutor` lists tutorials alphabetically, which puts `features-0.4` **before**
`getting-started`. The getting-started tutorial is the entry point (learn Nivis
from scratch) and should always come first; per-release feature tutorials follow
it. As more `features-<version>` tutorials accrue, alphabetical order also buries
the newest release's tutorial. The order should be controlled, not incidental.

## What changes
- `nivistutor`'s tutorial list (the `--list` output and the interactive menu) is
  ordered deterministically:
  1. **`getting-started` always first** (the entry-point tutorial).
  2. then `features-<version>` tutorials, **newest version first** (so the most
     recent release's tutorial is most prominent), comparing versions numerically.
  3. then any other tutorials alphabetically (a stable tiebreak).
- A new `features-<version>` tutorial slots into place by its version with no code
  change. The ordering lives in one comparison so it is easy to reason about.

## Non-goals
- A user-configurable order or per-tutorial weight metadata (the rule above is
  enough for the current set; revisit if a tutorial does not fit the scheme).
- Changing what a tutorial scaffolds, or the starter contents.

## Impact
- `cmd/nivistutor/tutorials.go`: `listTutorials` orders by the rule above instead
  of a plain name sort.
- Tests: a unit test asserting `getting-started` is first and `features-<version>`
  tutorials sort newest-first (with synthetic extra versions so the version
  comparison, not just the two current entries, is exercised).

Changelog: Changed the `nivistutor` tutorial menu order so getting-started is
always listed first, followed by the per-release feature tutorials newest-first.

Docs impact: none - the menu is self-describing and the GETTING-STARTED nivistutor
section does not enumerate a fixed order; no doc edit is required.
