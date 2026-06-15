---
# nixform2-f1yp
title: beans roadmap renders empty for todo milestones
status: scrapped
type: bug
priority: low
tags:
    - discovered
created_at: 2026-06-15T09:03:21Z
updated_at: 2026-06-15T15:34:57Z
parent: nixform2-hj4w
---

Discovered while bootstrapping (beans 1.4.1). `beans roadmap` and `beans roadmap --json` return no milestones even though a milestone with epic children exists in `beans list` (status=todo). Not blocking: `beans list` and `beans list --type epic --ready` work and are what the autonomous loop relies on. Investigate roadmap's milestone-status default/filter; cosmetic only.



## Reasons for Scrapping
Confirmed it is an UPSTREAM beans-tool bug, not terrae-nivis code, so there is
nothing to fix in this repo. Findings (2026-06-15):
- Originally filed against beans 1.4.1; current env is beans 0.4.2 (the CLI
  version changed between then and now).
- Reproduced in an isolated fresh beans dir: a milestone with status=todo renders
  NOTHING under `beans roadmap` (just '# Roadmap'). It only appears once the
  milestone is 'completed' — which is why our roadmap renders now (hj4w is done).
- The `--status todo` flag does NOT help — still empty — so it's a genuine bug,
  not a default-filter choice.
- Cosmetic and not relied upon: the autonomous loop uses `beans list` and
  `beans list --type epic --ready`, both of which work correctly.
Out of scope to fix here (would require an upstream beans issue/PR). Scrapping per
maintainer decision; reopen/file upstream only if beans roadmap becomes important.
