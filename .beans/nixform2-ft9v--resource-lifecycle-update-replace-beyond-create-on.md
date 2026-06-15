---
# nixform2-ft9v
title: 'Resource lifecycle: update & replace (beyond create-only)'
status: todo
type: epic
priority: normal
created_at: 2026-06-15T17:40:58Z
updated_at: 2026-06-15T17:42:22Z
---

Graduate the executor from create-only to a real apply loop: on a second apply with changed config, update in place (normal attrs) or replace = destroy+create (force-new attrs), instead of always creating a fresh resource. Generic across all resources (protocol/state layer; the provider decides create/update/replace). Driven by beans-faaf (user hit it: changing an S3 bucket name created a second bucket). OpenSpec change: resource-update-replace.


---
OpenSpec change scoped: resource-update-replace (openspec/changes/resource-update-replace) — validated, not yet implemented. Plan reads prior state; provider.PlanRequest gains PriorState; PlanResult surfaces RequiresReplace; v5/v6 encode prior state + read RequiresReplace; apply chooses create / update-in-place / replace(destroy+create). Generic at the protocol/state layer. Driving bug: beans-faaf.
