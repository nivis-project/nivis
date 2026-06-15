# Tasks: auto-build-derivations

## 1. Spec
- [x] 1.1 Write proposal, tasks, nix-lib + executor spec deltas (ADDED: __build leaf / nivis.drv; executor realises per-resource)
- [x] 1.2 `openspec validate auto-build-derivations` passes
- [x] 1.3 Document the `__build` leaf in docs/IR-CONTRACT.md (+ ir-schema if it enumerates leaves)

## 2. Nix lib
- [x] 2.1 `ref.nix`: `drv` helper -> `{ __build = { path = "${d}/${file or d.passthru.filePath}"; }; }`; `isBuild`; pass through `resolve` unchanged
- [x] 2.2 Export `drv` from `default.nix`; `toIR` emits the leaf in wire shape
- [x] 2.3 Nix property test: nivis.drv yields a well-formed __build leaf; survives resolve + toIR

## 3. Executor
- [x] 3.1 `ir` ingest: classify `__build` (a known leaf carrying a store path; not an unknown/edge)
- [x] 3.2 A realiser seam (interface) + real impl (`nix-store --realise <root>`); skip already-valid paths
- [x] 3.3 `applyOne`: walk resolved config, realise __build store roots, substitute the path; honor Driver.NoBuild
- [x] 3.4 `--no-build` flag on plan/apply (default: build)
- [x] 3.5 Build failure -> clear error naming the store path

## 4. Tests + migrate + live
- [x] 4.1 Go: applyOne realises + substitutes a __build leaf (stub realiser); --no-build skips; failure surfaces
- [x] 4.2 Migrate nix/example/ec2.nix to `nivis.drv` (drop manual passthru.filePath); drop the `nix build` step from the tutorial
- [x] 4.3 Full gate (build, go test, nix, IR conformance, mdbook, SSOT)
- [x] 4.4 Live: a single `nivis apply --attr nivis.ec2` builds the image + ships it -> HTTP 200 -> destroy clean (no orphan)

## 5. Close out
- [x] 5.1 `openspec archive auto-build-derivations`; fold into specs
- [x] 5.2 Close beans-qcwb; complete epic beans-uqn6; commit as Pim Snel; push
