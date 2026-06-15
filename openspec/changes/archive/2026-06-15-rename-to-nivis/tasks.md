# Tasks: rename-to-nivis

## 1. Spec
- [x] 1.1 Write proposal, tasks, branding spec delta (MODIFIED: product name -> Nivis + payoff; CLI is `nivis`/`nivis gen`)
- [x] 1.2 `openspec validate rename-to-nivis` passes

## 2. Module + CLI
- [x] 2.1 go.mod module -> github.com/wearetechnative/nivis; rewrite all import paths
- [x] 2.2 cmd/tn -> cmd/nivis (binary nivis)
- [x] 2.3 Fold cmd/tn-gen into a `nivis gen` cobra subcommand; remove cmd/tn-gen
- [x] 2.4 go build ./... + go test ./... pass

## 3. Flake
- [x] 3.1 flake.nix: terraeNivis.* -> nivis.* (default attr nivis.plan); apps/packages `nivis` (+ default)
- [x] 3.2 nix/example/* + nix/tests + driver default --attr -> nivis.plan
- [x] 3.3 nix run .#nivis -- --version, nivis gen, nix eval .#nivis.plan all work; lib still pure

## 4. Brand + docs + URLs
- [x] 4.1 Brand display Terrae Nivis -> Nivis + payoff "All your base belongs to Nix" (splash, README, docs, BRAND.md)
- [x] 4.2 README/docs/tutorial/INSTALL/RELEASING/DESIGN/CLAUDE + commands (tn -> nivis, tn-gen -> nivis gen)
- [x] 4.3 goreleaser + release.yml + docs.yml: owner/name + URLs -> nivis; og:image -> /nivis/
- [x] 4.4 docs-SSOT check fingerprints updated; mdbook build + check pass; goreleaser check passes

## 5. Close out
- [x] 5.1 Full gate (build, go test, nix, IR conformance, site, SSOT, goreleaser check); gofmt
- [x] 5.2 `openspec archive rename-to-nivis`; fold into branding spec
- [x] 5.3 Close beans-9pf9; advance epic; commit as Pim Snel
- [x] 5.4 DO NOT push — await GitHub repo rename; then `jj git remote set-url origin .../nivis` + push
