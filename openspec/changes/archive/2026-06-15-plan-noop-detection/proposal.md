# Proposal: plan-noop-detection

## Why
After `apply`, a `plan` with no config change keeps reporting `~ change`, and a
second `apply` re-applies every resource — which can fail (beans-l2q2: a re-applied
`aws_s3_object` trips a 404 in the provider's tagging read). Two gaps:

1. **The apply loop never detects a no-op.** `applyOne` always calls
   `ApplyResourceChange`, even when the planned state equals the prior state
   (nothing changed). Terraform/OpenTofu skip unchanged resources; TN does not, so
   every apply re-runs every resource and an unchanged update can error.
2. **`tn plan` lies.** It marks any resource already in state as `~` purely from
   "in state," without asking the provider whether anything actually differs — so
   it always says "needs change" even when it doesn't.

## What changes
- **Detect no-ops in the backend plan.** In the v5/v6 `Plan`, when there is prior
  state, decode the planned state and compare it to the prior state; if they are
  equal (and nothing is unknown-after-apply and no replace is required), report a
  **no-op**. Add `NoOp bool` to `provider.PlanResult`.
- **The driver skips a no-op.** `plan.Plan` gains `OpNoop`; `applyOne` short-
  circuits when the op is no-op — it does **not** call `ApplyResourceChange` (nor
  destroy), and re-uses the prior state's attributes for the outputs ledger so
  dependents still resolve. Create/update/replace are unchanged.
- **`tn plan` tells the truth.** Instead of blanket `~` for in-state resources, it
  plans each resource against its prior state and marks: `+` create, `~` update,
  `-/+` replace, and **`=` no change** (and reports the count of actual changes).
  This makes the post-apply plan show "no changes," matching reality.

## Non-goals
- Field-level diff rendering ("what changed") — we mark the operation, not a
  per-attribute diff. A richer plan view can come later.
- Refresh-before-plan / drift (beans-q3ku) — this compares planned config against
  *stored* state, not a fresh provider read. Out-of-band drift is still that
  bean's concern.
- Changing how create/update/replace behave (resource-update-replace) — only
  adding the no-op short-circuit.

## Impact
- Changed: `internal/provider/provider.go` (`PlanResult.NoOp`),
  `internal/provider/v5/v5.go` + `v6/v6.go` (compare planned vs prior),
  `internal/plan/plan.go` (`OpNoop` + render), `internal/phase/driver.go`
  (skip on no-op, ledger from prior), `cmd/tn/main.go` (`tn plan` plans per
  resource and marks `=`/`~`/`+`/`-/+`).
- Tests: a unit test that an unchanged prior==planned yields `NoOp` and `applyOne`
  calls neither Apply nor Destroy; a live AWS check that a second `apply` of the
  unchanged S3 example is a no-op (no 404) and `plan` shows no changes.
- Closes beans-l2q2; advances epic beans-ft9v.
