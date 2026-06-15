---
# nixform2-djay
title: E4c/4d Error UX & docs
status: completed
type: epic
priority: normal
tags:
    - off-critical-path
created_at: 2026-06-15T09:02:41Z
updated_at: 2026-06-15T11:50:43Z
parent: nixform2-hj4w
blocked_by:
    - nixform2-qv4t
---

Actionable errors with resource identity; README, getting-started on fake providers, IR contract & flake interface as documented contracts. OpenSpec changes: (record here).



OpenSpec changes: error-ux-docs.



## Summary of Changes
OpenSpec change error-ux-docs (archived 2026-06-15-error-ux-docs). The final epic.
- E4c error UX: cobra SilenceErrors + PersistentPreRun-set SilenceUsage so runtime
  errors print a clean 'error:' line with no usage dump (flag misuse still shows
  usage); cleanNixStderr extracts the Nix error: lines and drops the dirty-tree
  warning. Confirmed all four classes carry identity. CLI tests assert it.
- E4d docs: README rewritten (what nixform is, architecture, offline demo, layout,
  stable contracts, testing); docs/GETTING-STARTED.md hands-on walkthrough with
  real verified output; stable-contracts section points at IR-CONTRACT/ir-schema
  + the flake plan interface.
