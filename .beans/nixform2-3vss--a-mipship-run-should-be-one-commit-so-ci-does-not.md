---
# nixform2-3vss
title: A /mip:ship run should be one commit, so CI does not cancel its own run
status: todo
type: task
priority: low
tags:
    - discovered
created_at: 2026-09-08T13:21:50Z
updated_at: 2026-09-08T13:21:50Z
---

A `/mip:ship` run produces TWO commits seconds apart — the implementation, then the bean closure — and `.github/workflows/ci.yml` uses `concurrency: cancel-in-progress: true`, so the second push cancels the first commit's CI run. The head commit always passes, so nothing ships unverified, but no completed CI run ever exists against the implementation commit itself. Observed on both the `readable-provider-log-lines` and `realise-build-leaves-from-drv` ships.

## What is already done

`scripts/ship-change.sh` now takes `--bean <id>` (repeatable) and closes the bean itself, after the gate and before the commit, so a ship can be one commit. A failed gate leaves the bean untouched. It warns when a bean has no `## Summary of Changes`.

## What is left

`~/.claude/commands/mip:ship.md` still tells the agent to close the bean in step 5, *after* the ship — which recreates the second commit. Those command files are symlinks into the Nix store (home-manager), so they must be changed in the home-manager source, not in `~/.claude`.

The edit: replace step 4 and step 5 with

```
4. **Write the bean's summary** (not its status). Append a `## Summary of Changes`
   section to the linked bean's file. Leave the status alone — step 5 flips it,
   after the gate, so a failed gate leaves the bean untouched.

5. **Ship (gated).** Run:
   `bash scripts/ship-change.sh <change> "<commit subject>" --bean <id>`
   (repeat `--bean` per linked bean; omit when there is none). ...
   **A ship is ONE commit.** Do not close the bean, or make any other edit, in a
   separate commit afterwards: a second push seconds later cancels the first
   commit's CI run. Marking a now-complete parent milestone is a judgement call
   and may be a follow-up commit — that is the one exception.
```

`~/.claude/commands/mip:init.md` embeds the `ship-change.sh` template for new projects; it should grow the same `--bean` handling so a project scaffolded later inherits the one-commit behaviour.

## The alternative, if the skill edit is unwanted

Drop `cancel-in-progress` for pushes to `main` in `ci.yml` (keeping it for pull requests). Every commit on main then gets its own completed run, at the cost of some duplicate CI minutes. This is the cheaper fix and needs no change outside the repo.

## Acceptance criteria

- A `/mip:ship` run lands as a single commit on `main`, bean closure included.
- That commit has its own completed CI run.
