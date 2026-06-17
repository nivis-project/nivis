# Tasks: variables-and-overrides

## 1. IR contract (do first, the frozen-contract gate)
- [x] 1.1 `docs/IR-CONTRACT.md`: extend the outputs-ledger section with the
      optional `vars` object; state it is constant across phases, optional, and
      carries known data (no refs/unknowns).
- [x] 1.2 `docs/ir-schema.json`: N/A — the schema describes the IR *output*, not
      the ledger *input*; the ledger format lives in IR-CONTRACT.md prose only.
- [x] 1.3 Confirm `python3 tests/ir-conformance/check.py test` still passes
      (additive optional field).

## 2. Nix: mkVars
- [x] 2.1 `nix/lib/vars.nix` (or similar): `mkVars decls injected` resolving
      set/default/required, validating `str`/`int`/`bool`/`any`, throwing
      actionable errors naming the variable (and expected type). Pure; no IO.
- [x] 2.2 Export `mkVars` from `nix/lib/default.nix`.
- [x] 2.3 `nix/tests/properties.nix`: add a property covering set, default,
      required-throws, wrong-type-throws, and undeclared-ignored (the five
      scenarios in the nix-lib spec).

## 3. Go: ledger + resolution + flags
- [x] 3.1 `internal/ledger`: add `Vars map[string]interface{}` with
      `json:"vars,omitempty"` so an empty map omits the field.
- [x] 3.2 A var-resolution unit (e.g. `internal/vars`): merge sources lowest to
      highest = env `NIVIS_VAR_<name>` < `--var-file` (JSON, later wins) < `--var
      name=value` (later wins). Parse `--var` (split on first `=`), read+parse var
      files, actionable errors naming the offending input.
- [x] 3.3 `cmd/nivis`: add repeatable `--var` and `--var-file` persistent flags;
      resolve once; pass the resolved map into the phase driver.
- [x] 3.4 Phase driver / evaluator: set the resolved `vars` on the ledger every
      phase (constant across the fixpoint).

## 4. Tests
- [x] 4.1 Go table-driven (`internal/vars`): precedence (flag > file > env),
      later-wins within flags and within files, malformed `--var`, unreadable /
      non-object `--var-file`.
- [x] 4.2 Go: ledger marshals `vars` (and omits it when empty).
- [x] 4.3 Integration against the in-repo fakes: a `--var` resolves through a
      phase into a resource config (a fake resource whose config reads a var via
      `mkVars`), proving the end-to-end path.
- [x] 4.4 Nix property test (2.3) passes via `bash tests/run-nix-tests.sh`.

## 5. Docs
- [x] 5.1 AWS S3 tutorial: a short "parameterise with a variable" note
      (`mkVars` + `--var`), no em dashes.
- [x] 5.2 README Nix-library list: add `mkVars`.

## 6. Gate
- [x] 6.1 `gofmt`, `go build ./...`, `go test ./...` green.
- [x] 6.2 `bash tests/run-nix-tests.sh` (Nix properties + IR conformance) green.
- [x] 6.3 `bash tests/check-docs-ssot.sh` green (docs touched).
- [x] 6.4 `openspec validate variables-and-overrides --strict` passes; archive;
      update beans-kym5 (A1).
