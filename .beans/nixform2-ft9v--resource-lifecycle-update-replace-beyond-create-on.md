---
# nixform2-ft9v
title: 'Resource lifecycle: update & replace (beyond create-only)'
status: completed
type: epic
priority: normal
created_at: 2026-06-15T17:40:58Z
updated_at: 2026-06-15T20:38:45Z
---

Graduate the executor from create-only to a real apply loop: on a second apply with changed config, update in place (normal attrs) or replace = destroy+create (force-new attrs), instead of always creating a fresh resource. Generic across all resources (protocol/state layer; the provider decides create/update/replace). Driven by beans-faaf (user hit it: changing an S3 bucket name created a second bucket). OpenSpec change: resource-update-replace.


---
OpenSpec change scoped: resource-update-replace (openspec/changes/resource-update-replace) — validated, not yet implemented. Plan reads prior state; provider.PlanRequest gains PriorState; PlanResult surfaces RequiresReplace; v5/v6 encode prior state + read RequiresReplace; apply chooses create / update-in-place / replace(destroy+create). Generic at the protocol/state layer. Driving bug: beans-faaf.


---
First change landed: resource-update-replace (create/update/replace from prior state) — implemented + verified live against AWS, archived 2026-06-15. Remaining under this epic: beans-c2dx (prevent_destroy on replace — partially done: applyOne already refuses a replace when preventDestroy is set; bean can verify/extend), beans-q3ku (refresh-before-plan / drift).


---
EPIC COMPLETE (2026-06-15). All children done: faaf (create-only -> create/update/replace, resource-update-replace), l2q2 (no-op detection, plan-noop-detection), c2dx (prevent_destroy refuses replace — impl + unit + gated AWS e2e), q3ku (refresh-before-plan / drift + out-of-band-deletion handling, refresh-before-plan). The executor is now a real apply loop: create / update-in-place / replace / no-op, refreshing real state before planning, with --refresh opt-out and prevent_destroy honored. All proven against real AWS where applicable.
