# Tasks: readme-nix-first

## 1. Spec
- [x] 1.1 Write proposal, tasks, branding spec delta (MODIFIED: README is Nix-first; Go is a contributor note)
- [x] 1.2 `openspec validate readme-nix-first` passes

## 2. Rewrite
- [x] 2.1 Quickstart leads with `nix run …#nivis` + a fresh-flake snippet; links to getting-started/AWS tutorial
- [x] 2.2 Round-trip thesis kept; PoC framing softened to capability + a one-line status
- [x] 2.3 Go build moved to a short "Contributing / from source" note near the bottom
- [x] 2.4 README still links to canonical docs (no SSOT fingerprints copied in)

## 3. Verify + close
- [x] 3.1 `tests/check-docs-ssot.sh` passes; `mdbook build docs-site` ok
- [x] 3.2 README commands match the real CLI (nix run .#nivis, nivis plan)
- [x] 3.3 `openspec archive readme-nix-first`; fold into branding spec
- [x] 3.4 Close beans-tzd8; commit as Pim Snel; push
