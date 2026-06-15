# Tasks: nix-provider-config

## 1. Spec
- [x] 1.1 Write proposal, tasks, spec deltas (nix-lib ADDED + e2e MODIFIED)
- [x] 1.2 `openspec validate nix-provider-config` passes

## 2. Nix lib
- [x] 2.1 Add `nix/lib/mkProvider.nix`: `{ source, config ? {} }` → `{ source, config }`, error if `source` missing/non-string
- [x] 2.2 Export `mkProvider` from `nix/lib/default.nix`
- [x] 2.3 `toIR`: resolve+encode each provider's `config` (drop raw `inherit providers`); `source` passes through

## 3. AWS example + docs
- [x] 3.1 `nix/example/aws.nix`: use `mkProvider` with `region` (and `profile`) in config
- [x] 3.2 README + getting-started: provider config now lives in Nix; only credentials come from the environment

## 4. Tests
- [x] 4.1 Nix property test: provider config resolves `__ref`/`__derived` and encodes; `mkProvider` errors without `source`
- [x] 4.2 IR conformance still passes (provider config is valid IR)
- [x] 4.3 Go test: a provider config map incl. a nested block reaches `Configure`
- [x] 4.4 Full gate: `go build`, `go test ./...`, nix tests, IR conformance

## 5. Close out
- [x] 5.1 `openspec archive nix-provider-config`; fold requirements into specs
- [x] 5.2 Close beans-prj4; commit as Pim Snel; push
