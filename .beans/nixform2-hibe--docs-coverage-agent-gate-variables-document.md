---
# nixform2-hibe
title: Docs-coverage agent-gate + Variables document
status: completed
type: feature
tags:
    - discovered
    - roadmap
created_at: 2026-06-18T00:02:48Z
updated_at: 2026-06-18T00:02:48Z
parent: nixform2-zdj0
---

As features grow, "did the docs keep up?" must be a checkable step, not a thing an author remembers. Added a documentation-coverage gate and the first doc it would have demanded.

## Summary of Changes
- docs/DOCS-GATE.md: the rubric (new document / new paragraph / modifications / none) an author or agent runs at the end of every OpenSpec change, before archive. A new user-facing noun (variables, datasources, remote state) usually warrants its own doc.
- tests/check-docs-gate.sh: lightweight enforcement. It does NOT judge content (that's a judgment, not a regex); it fails if any in-scope OpenSpec change's proposal.md lacks a `Docs impact:` line, so the call can't be silently skipped. Pre-gate changes (archived on/before 2026-06-16) are exempt by date prefix. Chained into tests/check-docs-ssot.sh.
- CLAUDE.md: new section 5.5 "Documentation is part of done", the docs analogue of section 5 (testing).
- docs/VARIABLES.md: the canonical Variables reference (mkVars, types/defaults/required, --var/--var-file/NIVIS_VAR_*, Terraform precedence, purity/secrets, phased-eval fit, non-goals). Surfaced on the site: docs-site/src/variables.md {{#include}}s it + SUMMARY entry. This is the doc the gate would have demanded for A1 (variables-and-overrides), which had shipped only a tutorial paragraph.
- Retro-added `Docs impact:` lines to the two post-gate archived changes (nested-block-errors, variables-and-overrides) as worked examples.
- Trimmed the AWS S3 tutorial's variable section to a short note + link to VARIABLES.md (SSOT: no duplication).

Verified: docs gate green (SSOT + comparison-fresh + docs-coverage + mdbook build); the gate fails correctly when a Docs impact line is missing; no em dashes in authored prose.
