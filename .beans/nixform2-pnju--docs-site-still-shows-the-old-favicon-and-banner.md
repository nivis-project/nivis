---
# nixform2-pnju
title: Docs site still shows the old favicon and banner
status: completed
type: bug
priority: normal
tags:
    - discovered
created_at: 2026-11-19T14:38:43Z
updated_at: 2026-11-19T14:55:39Z
---

Found after shipping nixform2-nlzy: https://docs.nivis.tf/ shows the old twin-summit favicon and the old 'All your base belongs to Nix' banner. mdBook emits its own <link rel="icon" href="favicon.svg"> from theme/favicon.svg (the old glyph) after the one in head.hbs, and browsers use the last. The landing page src/index.md still shows banner.png. tests/check-docs-site.sh only read head.hbs, not the built page, so it missed the first. Change: docs-site-mark-and-hero.
