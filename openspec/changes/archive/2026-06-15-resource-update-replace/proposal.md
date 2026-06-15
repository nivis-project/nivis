# Proposal: resource-update-replace

## Why
The executor is **create-only**. Both Plan and Apply hardcode a *null*
`PriorState` to the provider (`internal/provider/v5/v5.go:146`, `:177`; v6
identical), `provider.PlanRequest` has no `PriorState` field, and the plan path
never reads the state store. So a **second `apply` with changed config does not
update or replace** the resource — the provider sees `PriorState = null`,
concludes "create," and makes a brand-new resource. A user hit exactly this
(beans-faaf): adding a `bucket` name to an already-applied `aws_s3_bucket` created
a *second* bucket and orphaned the first; the old state entry was silently
overwritten. `RequiresReplace` from the plan response is also discarded.

This was acceptable for the PoC (the milestone was the round trip — create +
read computed values back into Nix across phases — not lifecycle). It is not
acceptable for real use. This change graduates the executor to a real apply loop:
**update in place** when only normal attributes change, **replace**
(destroy-then-create) when a force-new attribute changes, and **create** only
when the resource is genuinely new.

The fix is **generic across all resources** — it lives entirely at the
protocol/state layer. The provider decides create/update/replace from
`(PriorState, ProposedNewState)` plus its own schema, exactly as Terraform/
OpenTofu do; there is no per-resource code. The needed primitives already exist:
state read-back (used by `destroy`/`refresh`), the value encode/decode codec, and
the destroy path.

## What changes
- **Plan reads prior state.** The phase driver / plan engine looks up the
  resource's existing state (`state.Store.Get(id)`) and passes its attributes as
  prior state into the plan. A genuinely-new resource (no state) keeps a null
  prior state.
- **`provider.PlanRequest` gains a `PriorState` field** (normalized attrs map,
  nil for create). The v5/v6 backends encode it with the existing `EncodeState`
  instead of always sending `priorNull`; null only when there is no prior state.
- **`provider.PlanResult` surfaces `RequiresReplace`.** The v5/v6 backends read
  `resp.GetRequiresReplace()` (the protocol already returns it; today it is
  discarded) and normalize it (the set of replaced attribute paths, or simply a
  boolean "replace required").
- **Apply decides create / update / replace** from `(prior state present?,
  RequiresReplace?)`:
  - *no prior state* → create (as today).
  - *prior state, no replace* → **update in place**: `ApplyResourceChange` with
    `PriorState = stored`, `PlannedState = planned`; persist the new attrs.
  - *prior state, replace required* → **destroy-then-create**: destroy the prior
    resource (the existing destroy path), then create the new one; persist the
    new attrs. No orphan.
- **`ApplyRequest` carries prior state** so the v5/v6 `ApplyResourceChange` call
  sends the real prior state (not `priorNull`) for an update.
- **State**: on replace, the old entry is replaced by the new one (already an
  upsert by id); the destroy removes the real prior resource first so nothing is
  orphaned.
- **Plan output** distinguishes create / update / replace (`+` / `~` / `-/+`) so
  `tn plan` tells the user what will happen.

## Non-goals
- `lifecycle` meta-args beyond what exists (`prevent_destroy`, `ignore_changes`)
  — those are a separate concern; this change is the create/update/replace
  mechanism. (It SHOULD honor `prevent_destroy` by refusing a replace, noted as a
  follow-up if not already wired.)
- Drift detection / `refresh`-before-plan — refresh already exists separately;
  this change plans against stored state, not a fresh read. Reconciling the two
  is a follow-up.
- Provider-side `UpgradeResourceState` / schema-version migrations.
- Move/import.

## Impact
- Changed: `internal/provider/provider.go` (`PlanRequest.PriorState`,
  `PlanResult.RequiresReplace`, `ApplyRequest.PriorState`),
  `internal/provider/v5/v5.go` + `v6/v6.go` (encode prior state; read
  RequiresReplace), `internal/plan/plan.go` + `internal/phase/driver.go` (read
  state, thread prior state, choose path), `internal/apply/apply.go` (update vs.
  replace), plan rendering. No per-resource code; no IR/contract change.
- Tests: fake-provider update (attr changes in place) and replace (a force-new
  attr triggers destroy+create) cases; a "plan shows update/replace" assertion;
  a gated AWS **apply-twice** e2e (change a tag in place; change `bucket` to force
  a replace — old bucket gone, one bucket remains).
- Closes beans-faaf; advances the lifecycle epic (beans-ft9v).
