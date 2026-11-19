---
# nixform2-24s4
title: Docs landing image is stretched
status: completed
type: bug
priority: normal
tags:
    - discovered
created_at: 2026-11-19T15:16:58Z
updated_at: 2026-11-19T15:57:05Z
---

Found by Pim after shipping nixform2-pnju: the landing image on https://docs.nivis.tf/ is stretched vertically. The img carries width=1200 height=630 and .tn-hero img sets max-width: 100% without height: auto, so the width shrinks and the height stays 630. Change: docs-hero-aspect-ratio.
