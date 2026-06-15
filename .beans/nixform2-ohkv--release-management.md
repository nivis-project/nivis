---
# nixform2-ohkv
title: release management
status: completed
type: task
priority: normal
created_at: 2026-06-15T16:17:25Z
updated_at: 2026-06-15T18:59:51Z
parent: nixform2-esyd
---

- changelog management
- semantic versioning
- single source of truth for app-version
- easy to handle script: patch/minor/major
- jj/git compatible


---
DONE via OpenSpec change release-management (archived 2026-06-15-release-management). All five asks delivered: (1) single source of truth = top-level VERSION file (0.2.0; reset from the bogus v1.0 — 0.1.x was the PoC); flake.nix reads it (lib.fileContents) + injects via -ldflags -X main.version, plain go build honestly reports "dev". Verified nix run/go-build-with-ldflags both show 0.2.0. (2) CHANGELOG.md (Keep-a-Changelog) with a 0.1.0 PoC summary + Unreleased. (3) scripts/release.sh patch|minor|major [--dry-run]: bumps VERSION, rolls Unreleased into a dated section, commits (jj), tags v<x>, pushes — jj/git compatible; dry-run verified (0.2.0->0.3.0/0.2.1/1.0.0). (4) .goreleaser.yaml + .github/workflows/release.yml: on tag v*, goreleaser cross-builds tn+tn-gen (linux/darwin × amd64/arm64), checksums, notes, GitHub release. goreleaser check passes; a --snapshot build produced all 4 platforms' archives. (5) docs/RELEASING.md. No real tag cut yet — awaiting explicit go-ahead.
