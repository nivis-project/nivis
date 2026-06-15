---
# nixform2-807d
title: step by step tutorial for AWS s3 bucket
status: completed
type: task
priority: normal
created_at: 2026-06-15T16:38:22Z
updated_at: 2026-06-15T17:14:25Z
---

---
DONE via OpenSpec change aws-s3-tutorial (archived 2026-06-15-aws-s3-tutorial). Added docs/TUTORIAL-AWS-S3.md: from-scratch step-by-step (prereqs, get tn, creds, the config explained line by line, plan/apply/state/destroy, troubleshooting). VERIFIED LIVE against AWS (account 076504012268): apply created bucket terraform-20260615165907082000000001, state show confirmed the round trip (tags_all merged default_tags), destroy removed it, no orphan — every command/output in the tutorial is real. SSOT respected: the full walkthrough lives once in the tutorial; getting-started §7 trimmed to an intro + link; docs-ssot check + canonical table updated; site page docs-site/src/TUTORIAL-AWS-S3.md (named to match the link target so it resolves in-repo AND on the site) + SUMMARY nav. Discovered + fixed a bug in nix/example/aws.nix along the way (beans-5ifi: default_tags must be a list).


---
Reopened: the first cut assumed you were inside the terrae-nivis repo (go build ./cmd/tn, --attr terraeNivis.aws). Reworking to be genuinely from-scratch on a user's own machine: (1) docs/INSTALL.md — global TN install (nix run/shell/profile/go); (2) rewritten docs/TUTORIAL-AWS-S3.md — nix flake init a fresh infra flake that consumes github:wearetechnative/terrae-nivis as an input and exposes terraeNivis.plan, then add the S3 resource, driven by bare `tn plan/apply/...`. Verified an external flake consuming TN works (path: input + bare tn plan). OpenSpec: docs-tutorial-rework.


---
Reworked + DONE via OpenSpec change docs-tutorial-rework (archived 2026-06-15-docs-tutorial-rework). Now genuinely from-scratch: Part 1 installs tn (links docs/INSTALL.md — new: nix run/shell/profile/go); Part 2 scaffolds a fresh flake with `nix flake init`, replaces flake.nix with boilerplate that inputs github:wearetechnative/terrae-nivis, binds tn = terrae-nivis.lib, exposes terraeNivis.plan (the default attr), and declares the S3 bucket; Part 3 runs bare tn plan/apply/state/destroy from the user's own dir. Verified live: an external flake (path: input) + `nix flake init` + bare `tn plan` works; the apply/state/destroy half was proven against real AWS in the prior cut. SSOT: INSTALL is canonical (guarded by the check); the AWS walkthrough stays canonical in the tutorial; site gets an Install page + nav. No CLI/flake code change needed — the consuming-flake flow already worked.
