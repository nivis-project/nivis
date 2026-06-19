# Tasks: nivistutor-menu-order

## 1. Controlled ordering
- [x] 1.1 `cmd/nivistutor/tutorials.go`: replace the plain name sort in
      `listTutorials` with a comparator that ranks `getting-started` first, then
      `features-<version>` newest-version-first (numeric version compare), then any
      others alphabetically.
- [x] 1.2 Keep it deterministic and self-extending: a new `features-<version>`
      slots in by version with no further code change.

## 2. Test
- [x] 2.1 A unit test on the ordering: with `getting-started` plus several
      `features-<version>` names (e.g. 0.4, 0.5, 0.10), assert getting-started is
      first and the features sort newest-first (0.10 > 0.5 > 0.4, i.e. a numeric,
      not lexical, version compare).

## 3. Changelog
- [x] 3.1 `CHANGELOG.md` `[Unreleased]` `### Changed`: the menu-order change
      (matches the proposal's `Changelog:` line).

## 4. Gate
- [x] 4.1 `gofmt`, `go build ./...`, `go test ./...` green.
- [x] 4.2 `bash tests/run-nix-tests.sh` + `bash tests/check-docs-ssot.sh` green.
- [x] 4.3 `openspec validate nivistutor-menu-order --strict`; archive; close
      beans-x3v1.
