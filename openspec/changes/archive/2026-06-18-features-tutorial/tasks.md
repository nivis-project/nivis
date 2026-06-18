# Tasks: features-tutorial

## 1. Fix: mkVars string->scalar coercion
- [x] 1.1 `nix/lib/vars.nix`: coerce a string int/bool value to the declared type
      (regex-guard int parse for a clean error; true/false for bool); typed
      passthrough otherwise.
- [x] 1.2 `nix/tests/properties.nix`: P9 extended ("5"->5, "-3"->-3, true/false,
      bad int throws).

## 2. Fix: ResolveOutputs re-reads datasources
- [x] 2.1 `internal/phase/driver.go`: after the state-seeded eval, read ready
      datasources, add to the ledger, re-eval, then collect outputs.
- [x] 2.2 `internal/phase/outputs_e2e_test.go`: TestStackOutputsResolveDatasource
      guards that a datasource-derived output resolves standalone.

## 3. Tutorial config + flake attr
- [x] 3.1 `nix/example/tutorial.nix`: vars + datasource + round trip + outputs.
- [x] 3.2 `flake.nix`: expose it as `nivis.tutorial`.

## 4. The tutorial doc
- [x] 4.1 `docs/TUTORIAL-FEATURES.md`: hands-on walkthrough with verified output.
- [x] 4.2 `docs-site/src/TUTORIAL-FEATURES.md` include + SUMMARY entry.

## 5. Gate
- [x] 5.1 `gofmt`, `go build`, `go test ./...` green.
- [x] 5.2 `bash tests/run-nix-tests.sh` green; `bash tests/check-docs-ssot.sh` green.
- [x] 5.3 `openspec validate features-tutorial --strict`; archive.
