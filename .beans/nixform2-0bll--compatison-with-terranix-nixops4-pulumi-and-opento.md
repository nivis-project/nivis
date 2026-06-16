---
# nixform2-0bll
title: compatison with Terranix, Nixops4, Pulumi and OpenTofu
status: completed
type: task
priority: normal
created_at: 2026-06-16T10:23:31Z
updated_at: 2026-06-16T10:34:14Z
---

comparison with Terranix, Nixops4, Pulumi and OpenTofu, CDK, CloudFormation etc...

We need an dedicated page in the docs-site in which we compare NIVIS with all
usual suspects. It should be an honest comparison and also list essential
features and enterprice features. This comparison should show in which way
Nivis is a unique project but it should also show the maturity of NIVIS
compared to the others. Online are propably existing tables.


---
DONE (no OpenSpec proposal — docs change). Added docs/COMPARISON.md: an honest comparison of Nivis vs Terranix, OpenTofu/Terraform, NixOps 4, Pulumi, CDK and CloudFormation — one-line positioning, what makes Nivis genuinely different (first-class Nix values / spawn-not-link / round-trip via phased re-eval), essential + enterprise/operational feature tables, a licensing table (no BUSL), a "when to pick what" guide, and explicit honesty about Nivis being alpha. Surfaced as a site page (docs-site/src/comparison.md {{#include}}s the canonical doc, added to SUMMARY) and linked from README, following the docs SSOT pattern.

FRESHNESS MECHANISM (the added requirement): the page carries a `last-verified: YYYY-MM-DD` marker and a Sources section of upstream links. tests/check-comparison-fresh.sh (hermetic, date math only) fails once the marker is older than MAX_AGE_DAYS (default 180), in the future, or missing, and is chained into tests/check-docs-ssot.sh so the docs gate catches a stale comparison. Verified: passes today; an old date trips it (897 > 180); a future date is caught. To re-verify: check the Sources, then bump last-verified.
