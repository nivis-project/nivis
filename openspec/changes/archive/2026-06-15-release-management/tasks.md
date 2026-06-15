# Tasks: release-management

## 1. Spec
- [x] 1.1 Write proposal, tasks, release spec delta (ADDED: version SoT, changelog, release script, release CI)
- [x] 1.2 `openspec validate release-management` passes

## 2. Version single source of truth
- [x] 2.1 Add `VERSION` = `0.2.0`
- [x] 2.2 Go: `version` default from embedded `VERSION` (`//go:embed`); used by `--version` + splash
- [x] 2.3 `flake.nix`: read `VERSION`, set package version + inject `-ldflags -X main.version`
- [x] 2.4 Verify `nix run .#tn -- --version` and `go build` both report `0.2.0`

## 3. Changelog + release script
- [x] 3.1 `CHANGELOG.md` (Keep-a-Changelog): Unreleased + 0.1.0 (PoC summary)
- [x] 3.2 `scripts/release.sh patch|minor|major [--dry-run]`: bump VERSION, roll changelog, commit, tag, push (jj/git)
- [x] 3.3 `scripts/release.sh --dry-run` shows a correct next-version plan

## 4. goreleaser + CI
- [x] 4.1 `.goreleaser.yaml`: build tn + tn-gen (linux/darwin, amd64/arm64), version from tag, checksums, changelog notes
- [x] 4.2 `.github/workflows/release.yml`: on tag `v*`, run goreleaser to create the GitHub release
- [x] 4.3 `goreleaser check` passes; a `goreleaser --snapshot --clean` build produces the binaries

## 5. Close out
- [x] 5.1 `docs/RELEASING.md`; full gate (build, go test, nix, IR conformance)
- [x] 5.2 `openspec archive release-management`; fold into release spec
- [x] 5.3 Close beans-ohkv; advance epic beans-esyd; commit as Pim Snel; push (NO real tag without explicit go-ahead)
