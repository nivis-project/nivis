---
# nixform2-jf7y
title: Bump docs-deploy actions off deprecated Node 20
status: completed
type: task
priority: low
tags:
    - discovered
created_at: 2026-06-15T16:15:52Z
updated_at: 2026-06-15T16:16:01Z
parent: nixform2-oeri
---

The Pages deploy workflow pinned actions (checkout@v4, cache@v4, configure-pages@v5, upload-pages-artifact@v3, deploy-pages@v4) ran on Node 20, which GitHub deprecated (forced to Node 24 from 2026-06-16). Bumped to checkout@v6, cache@v5, configure-pages@v6, upload-pages-artifact@v5, deploy-pages@v5 (all confirmed using: node24). No requirement change; the docs-deploy 'site is deployed' requirement still holds.
