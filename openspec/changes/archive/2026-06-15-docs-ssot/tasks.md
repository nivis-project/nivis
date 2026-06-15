# Tasks: docs-ssot

## 1. Spec
- [x] 1.1 Write proposal, tasks, branding spec delta (ADDED docs single-source-of-truth requirement)
- [x] 1.2 `openspec validate docs-ssot` passes

## 2. Designate canonical owners + extract fragments
- [x] 2.1 Decide the canonical file per shared topic (AWS walkthrough, how-it-works, build/run) and record it in `docs-site/README.md`
- [x] 2.2 Extract shared fragments / add mdBook include anchors where a section must render in >1 place (`docs/_fragments/` or anchored ranges)

## 3. De-duplicate
- [x] 3.1 AWS walkthrough: site `real-providers.md` → `{{#include}}` of the canonical section; README → teaser + link
- [x] 3.2 How-it-works/round-trip: one canonical copy; `index.md` + getting-started link to it; README keeps only the headline paragraph
- [x] 3.3 Build/run commands: canonical in getting-started; README shows minimal quickstart + link

## 4. Enforce + verify
- [x] 4.1 Add a docs-SSOT check under `tests/` (fails on verbatim duplication of a canonical block; asserts the site builds)
- [x] 4.2 `mdbook build docs-site` succeeds; the SSOT check passes; spot-check no rendered page lost content
- [x] 4.3 `openspec archive docs-ssot`; fold requirement into branding spec
- [x] 4.4 Close beans-qvx3; commit as Pim Snel; push
