# Tasks: plan-apply-output-fidelity

## 1. Apply reports the real op (beans-z57y)
- [x] 1.1 `internal/phase/driver.go`: `AppliedNode` gains an `Op plan.Op` field
      (zero value / unused for datasources, which stay `IsData`).
- [x] 1.2 `applyOne` returns the resolved `plan.Op` (alongside the outputs), or the
      driver records the op when it appends the node, so the apply loop can set
      `AppliedNode.Op`. No change to which nodes apply or the order.
- [x] 1.3 `cmd/nivis/main.go` `applyCmd`: render each applied node by its op
      (`+` create, `~` update, `-/+` replace, `=` no-op), datasource stays `r`
      read, using the existing `newOutput` markers. Keep the count summary.

## 2. Plan reads datasources (beans-oh90)
- [x] 2.1 `internal/phase/driver.go` `PlanReport`: before resolving TF→TF refs,
      read each datasource whose inputs are known (reusing the same readiness +
      `readOne` path the apply loop uses) and seed its outputs into the ledger,
      iterating to a fixpoint if a datasource depends on a resource already in
      state. Datasources are still not planned/applied/stored.
- [x] 2.2 A resource whose config depends only on a datasource is then
      `FullyKnown`, so it is planned against the provider and reports its true op
      (no-op when unchanged) instead of the `else if found → OpUpdate` fallback.

## 3. Unit tests
- [x] 3.1 `internal/phase`: a `PlanReport` test with a datasource feeding a
      resource (resource already in state, unchanged) asserts the resource is
      reported `OpNoop`, not `OpUpdate`.
- [x] 3.2 `internal/phase`: the apply result's `AppliedNode`s carry the resolved
      op (create on first apply; no-op/update on a re-apply of unchanged config).

## 4. Hermetic e2e (the regression guard)
- [x] 4.1 `tests/e2e`: against the fakes, a config shaped like the tutorial
      (a datasource `alpha_lookup` feeding `alpha_token`, and `beta_record`
      depending on the token). Apply once, then:
      - second `plan` (PlanReport) reports the datasource-dependent resource as
        no-op (`=`), NOT `~ update`;
      - second `apply` reports the resources as no-op/update (NOT `+ create`), and
        the stored ids/computed values are unchanged across the two applies (so it
        is a real no-op, not a re-create).

## 5. Changelog
- [x] 5.1 `CHANGELOG.md` `[Unreleased]` `### Fixed`: both fixes (matches the
      proposal's `Changelog:` line).

## 6. Gate
- [x] 6.1 `gofmt`, `go build ./...`, `go test ./...` green.
- [x] 6.2 `bash tests/run-nix-tests.sh` + `bash tests/check-docs-ssot.sh` green.
- [x] 6.3 `openspec validate plan-apply-output-fidelity --strict`; archive; close
      beans-z57y and beans-oh90.
