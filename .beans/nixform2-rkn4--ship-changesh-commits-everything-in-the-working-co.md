---
# nixform2-rkn4
title: ship-change.sh commits everything in the working copy, including another session's work
status: todo
type: bug
priority: high
tags:
    - discovered
created_at: 2026-09-15T15:44:59Z
updated_at: 2026-09-15T15:44:59Z
---

Discovered on 2026-09-15 while shipping `nix-logformat-conformance`
(bean nixform2-lpqk). It happened for real; it was not a near miss.

## What happened

Commit `2a03c89`, titled `beans: schema defaults never reach a provider`,
contained one beans file belonging to that work — and thirteen files belonging
to an unrelated, in-progress change:

    .beans/nixform2-1mk0--unset-optional-computed-attributes...md   <- its own work
    .beans/nixform2-irf4--claudemd-section-6-wrongly-states...md    <- someone else's
    .beans/nixform2-lpqk--ci-nix-version-matrix...md                <- someone else's
    .github/workflows/ci.yml                                        <- someone else's
    internal/nixlog/conformance_fresh_test.go                       <- someone else's
    internal/nixlog/conformance_test.go                             <- someone else's
    internal/nixlog/nixlog_test.go                                  <- someone else's
    internal/nixlog/testdata/capture.sh                             <- someone else's
    internal/nixlog/testdata/realise-chain.jsonl                    <- someone else's
    internal/nixlog/testdata/realise-nix_2_2{8}.jsonl               <- someone else's
    internal/nixlog/testdata/realise-nix_2_3{0,1,4}.jsonl           <- someone else's
    internal/phase/consume_test.go                                  <- someone else's
    tests/check-nix-logformat.sh                                     <- someone else's

Recovered by `jj split` before it was pushed. Nothing was lost.

## Why

`scripts/ship-change.sh` stages the ENTIRE working copy, twice:

    line  86:  git add -A        # [1/7] stage working tree
    line 117:  git add -A        # [5/7] commit
    line 118:  jj commit -m "${SUBJECT}"

With two sessions in one working copy, whichever ships first sweeps up every
uncommitted file the other has in flight, and labels it with its own subject.

## Why it is worse than it looks

It was caught only because the title was obviously unrelated to the contents. A
ship whose subject happened to sound plausible would have been pushed without
anyone looking, and the two changes would be entangled in published history —
where `jj split` is no longer the easy answer.

It also defeats the gate's meaning. `nix flake check` then runs over a tree
containing another change's half-finished work, so a green gate says nothing
about either change on its own. Twice today the gate failed on code that was
not part of the change being shipped.

## The fix pattern already exists in the sibling script

`scripts/release.sh:87` stages exactly what it owns:

    git add VERSION CHANGELOG.md

`ship-change.sh` could do the same: stage the paths the change actually
touches. The change's own OpenSpec artifacts name much of it, and
`openspec show <change> --json` plus the bean file is a starting point.

## The constraint that makes this non-trivial

The comment at line 82 explains why `git add -A` is there:

> jj snapshots the working copy on its own, but `nix flake check` evaluates the
> GIT tree: a file git does not know about is invisible to the flake. Staging is
> how new files become visible.

So the staging is not incidental — an unstaged new file would simply not be
seen by the gate, and the gate would pass while testing less than it appears
to. Any fix must keep new files visible to the flake while not claiming files
the change does not own.

## Directions, none obviously right

1. **Stage a computed path set.** Precise, but needs a reliable way to know
   what the change touches, and getting it wrong reintroduces the
   invisible-file problem quietly.
2. **Refuse to ship a dirty tree that was dirty before the run.** Snapshot the
   working-copy state at `[1/7]`, and abort if it contains anything the change
   is not expected to touch. Blunt, but it fails loudly instead of silently,
   and a human can then decide.
3. **Ship from a fresh jj workspace.** `jj workspace add` gives an isolated
   working copy per ship, which removes the sharing entirely. Heaviest, and
   changes how the script is invoked.

Direction 2 looks like the best value for the effort: it does not need to know
what a change owns, only that the tree held surprises.

## Related

Same family as the gates that are green when they should be red:
`nixform2-pex6` (changelog gate cannot fail until after archiving) and
`nixform2-p72l` (docs gate skips tinychanges). Here the failure is a commit
that looks correct and is not.
