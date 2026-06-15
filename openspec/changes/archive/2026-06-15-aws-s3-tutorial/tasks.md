# Tasks: aws-s3-tutorial

## 1. Spec
- [x] 1.1 Write proposal, tasks, e2e spec delta (ADDED: a verified from-scratch AWS S3 tutorial exists)
- [x] 1.2 `openspec validate aws-s3-tutorial` passes

## 2. Write the tutorial
- [x] 2.1 `docs/TUTORIAL-AWS-S3.md`: prereqs → get tn → creds → write bucket.nix (explained) → plan/apply/inspect → destroy → troubleshooting
- [x] 2.2 SSOT: trim getting-started §7 to a brief intro + link to the tutorial (stop repeating the command set)

## 3. Verify live + wire into site
- [x] 3.1 Run the exact tutorial steps against real AWS (AWS_PROFILE) — create + destroy one bucket, confirm no orphan; paste the real outputs
- [x] 3.2 `docs-site/src/tutorial-aws-s3.md` ({{#include}}) + `SUMMARY.md` nav entry
- [x] 3.3 Update docs-ssot canonical table (docs-site/README.md) + `tests/check-docs-ssot.sh` to register/guard the tutorial as canonical
- [x] 3.4 `mdbook build docs-site` succeeds; `tests/check-docs-ssot.sh` passes

## 4. Close out
- [x] 4.1 `openspec archive aws-s3-tutorial`; fold requirement into e2e spec
- [x] 4.2 Close beans-807d; commit as Pim Snel; push
