# Proposal: plan-apply-output-fidelity

## Why
Two reporting bugs surfaced by the features-0.4 tutorial make `nivis plan`/`apply`
output disagree with what the executor actually did. The underlying state machine
is correct (ids and computed values are stable across re-applies; nothing is
re-created), but the CLI lies:

- **`nivis apply` always prints `+ create`** for every applied resource, even when
  it was an in-place update or a no-op (beans-z57y). `applyOne` computes and acts
  on the right `plan.Op`, but the result type (`AppliedNode`) does not carry it, so
  `applyCmd` hardcodes `out.create(...)`.
- **`nivis plan` reports a spurious `~ update`** for a resource whose config
  references a **datasource** output (beans-oh90). `PlanReport` seeds the ledger
  only from stored state; datasources are never stored, so a datasource is not
  read during plan, its dependents are not `FullyKnown`, and the plan loop falls
  into an "in state but not resolvable → update pending" fallback. In the tutorial,
  `alpha.alpha_token.app` (`label` from `data.alpha.alpha_lookup.existing`) shows
  `~ update` on every re-plan, while `beta.beta_record.app` correctly shows `=`
  (its dependency is a stored resource, not a datasource).

Both are about plan/apply output telling the truth, so they are fixed together.

## What changes
- **Apply reports the real op.** The phase driver threads each applied resource's
  resolved operation (create / update / replace / no-op) out of `applyOne` into
  the apply result (`AppliedNode`). `nivis apply` renders that op with the same
  marker vocabulary as plan (`+` / `~` / `-/+` / `=`), and a datasource stays `r`
  read. This is reporting metadata only: which nodes apply, the order, and apply
  behaviour are unchanged.
- **Plan reads datasources.** `PlanReport` reads each side-effect-free datasource
  (when its inputs are known) and seeds its outputs into the ledger before
  resolving TF→TF refs, exactly as the apply loop already does. A resource whose
  config depends only on a datasource then resolves and is planned against the
  provider, so it reports its true op (`=` when unchanged) instead of the
  spurious `~`. Datasources are still never planned, applied, or written to state.

## Non-goals
- Changing apply/plan behaviour (what is created/updated/destroyed), the phase
  order, or the fixpoint loop. This is a reporting-fidelity fix.
- Changing how datasources are read during apply (already correct).

## Impact
- `internal/phase/driver.go`: `AppliedNode` gains the resolved `Op`; `applyOne`
  returns its op (or the driver records it); `PlanReport` reads datasources into
  the ledger before resolving.
- `cmd/nivis/main.go`: `applyCmd` renders the per-node op rather than always
  `create`.
- Tests: a unit test that `PlanReport` reports `=`/no-op (not `~`) for a
  datasource-dependent resource that is unchanged; and a hermetic **e2e** against
  the fakes (a datasource feeding a resource, like the tutorial) that:
  - a second `plan` reports the datasource-dependent resource as no-op (`=`), not
    update;
  - a second `apply` reports it (and a downstream resource) as no-op/update, not
    `create`, and the stored ids/values are unchanged across applies.

Changelog: Fixed `nivis apply` always printing `+ create` (it now shows the real
op: create/update/replace/no-op) and `nivis plan` reporting a spurious `~ update`
for a resource whose config reads a datasource (plan now reads datasources, so an
unchanged datasource-dependent resource reports no-op).

Docs impact: none - a plan/apply output-fidelity fix; no new user-facing concept
or capability, and the documented markers (`+`/`~`/`-/+`/`=`/`r`) are unchanged
(apply now uses them correctly). The getting-started/feature tutorials already
describe re-plan/re-apply; no doc edit is required.
