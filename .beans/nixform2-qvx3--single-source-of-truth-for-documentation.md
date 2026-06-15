---
# nixform2-qvx3
title: single source of truth for documentation
status: completed
type: task
priority: normal
created_at: 2026-06-15T16:40:17Z
updated_at: 2026-06-15T16:48:22Z
---

---
Scoped as OpenSpec change docs-ssot (openspec/changes/docs-ssot). Problem confirmed: the AWS "Real providers" walkthrough is duplicated across README + getting-started + the site's real-providers.md; the "how it works"/round-trip intro across README + index.md + getting-started; the build/demo command block across README + getting-started — and it already drifted (the prj4/docs-deploy work had to patch the same text in 3 files). Plan: one canonical owner per topic (recorded in docs-site/README.md), others {{#include}} or link, plus a tests/ docs-SSOT check that fails on verbatim duplication and asserts the site still builds. Not started.


---
DONE via OpenSpec change docs-ssot (archived 2026-06-15-docs-ssot). One canonical source per shared topic, recorded in docs-site/README.md: pitch + how-it-works -> docs/OVERVIEW.md (new, mdBook anchors pitch/how-it-works); fake-provider walkthrough + build/run + AWS (anchor: aws) -> docs/GETTING-STARTED.md; contract/testing/design/roadmap/brand -> their docs/ files. Site pages now {{#include}} those (index.md includes OVERVIEW anchors; real-providers.md includes GETTING-STARTED §7) instead of bespoke copies. README trimmed to hero + pitch + minimal quickstart + links (GitHub can't process mdBook includes, so README links rather than includes). Added tests/check-docs-ssot.sh: fails if a canonical fingerprint is copied into README/site framing pages, and asserts mdbook build resolves all includes — negative-tested (catches an injected dup). Verified: site builds (11 pages), content census shows nothing lost, full check green.
