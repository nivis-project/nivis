---
# nixform2-esyd
title: Release management (versioning, changelog, releases)
status: todo
type: epic
priority: normal
created_at: 2026-06-15T18:52:16Z
updated_at: 2026-06-15T18:59:51Z
---

Single source of truth for the app version (a VERSION file read by both the flake and the Go binary), a Keep-a-Changelog CHANGELOG, a jj/git-compatible scripts/release.sh (patch/minor/major: bump VERSION + roll changelog + commit + tag + push), and a goreleaser GitHub Action that cuts a multi-platform GitHub release on a v* tag. Resets the version from the bogus v1.0 to 0.2.0 (0.1.x was the PoC). Driven by beans-ohkv. OpenSpec change: release-management.


---
First (and core) change landed: release-management — VERSION SoT (0.2.0), CHANGELOG, scripts/release.sh, goreleaser CI on tag, docs/RELEASING.md. Archived 2026-06-15. The machinery is in place and verified (goreleaser check + snapshot build, release.sh dry-run); cutting an actual v0.2.0 release is a deliberate user-triggered step (run scripts/release.sh minor/major or push a v* tag).
