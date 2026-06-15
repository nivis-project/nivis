---
# nixform2-807d
title: step by step tutorial for AWS s3 bucket
status: completed
type: task
priority: normal
created_at: 2026-06-15T16:38:22Z
updated_at: 2026-06-15T17:02:37Z
---

---
DONE via OpenSpec change aws-s3-tutorial (archived 2026-06-15-aws-s3-tutorial). Added docs/TUTORIAL-AWS-S3.md: from-scratch step-by-step (prereqs, get tn, creds, the config explained line by line, plan/apply/state/destroy, troubleshooting). VERIFIED LIVE against AWS (account 076504012268): apply created bucket terraform-20260615165907082000000001, state show confirmed the round trip (tags_all merged default_tags), destroy removed it, no orphan — every command/output in the tutorial is real. SSOT respected: the full walkthrough lives once in the tutorial; getting-started §7 trimmed to an intro + link; docs-ssot check + canonical table updated; site page docs-site/src/TUTORIAL-AWS-S3.md (named to match the link target so it resolves in-repo AND on the site) + SUMMARY nav. Discovered + fixed a bug in nix/example/aws.nix along the way (beans-5ifi: default_tags must be a list).
