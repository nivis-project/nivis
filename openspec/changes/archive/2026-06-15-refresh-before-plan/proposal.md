# Proposal: refresh-before-plan

## Why
`plan` and `apply` currently plan each resource against its **stored** state
(`store.Get`), never re-reading the provider. So out-of-band drift is invisible:
if someone changes a bucket in the AWS console, or deletes it, Nivis plans
against its own stale record and can produce a wrong plan (e.g. "no change" when
the real resource is gone, or applying against attributes that no longer exist).
Terraform/OpenTofu solve this by refreshing (`ReadResource`) before planning;
Nivis has the `refresh` primitive (`internal/refresh`) but doesn't use it in the
plan/apply path (beans-q3ku, the last open piece of the lifecycle epic ft9v).

## What changes
- **Refresh before planning, by default.** Before planning an in-state resource,
  `PlanReport` and the apply loop `Read` it through the provider and use the
  **fresh** attributes as the prior state, so the plan reflects reality:
  - read returns attributes → plan against those (drift is seen; an unchanged
    resource is still a no-op, a drifted one shows the diff),
  - read returns **empty** (the resource was deleted out-of-band) → treat as no
    prior state → plan/apply it as a **create** (re-create the missing resource),
  - a genuinely new resource (no stored state) is unchanged: still a create, no
    read.
- **An opt-out.** A `--refresh` flag (default `true`) on `plan`/`apply`, backed
  by a `Driver` option, so a user can `--refresh=false` to plan against stored
  state only (faster, offline) — matching Terraform's `-refresh=false`.
- **`apply` persists the refreshed state** for resources it touches (it already
  writes post-apply state); a no-op resource whose read showed drift has its
  state updated to the read result.

## Non-goals
- A standalone "drift report" / `plan -detailed-exitcode` — just feed reads into
  the existing plan. Reporting niceties can come later.
- `ignore_changes` handling — separate lifecycle meta; not this change.
- Changing the `refresh` command itself (it already reconciles state); this wires
  the same Read into plan/apply.

## Impact
- Changed: `internal/phase/driver.go` (`PlanReport` + `applyOne` read prior state
  via the provider when refresh is on; `Driver.NoRefresh` option), `cmd/nivis`
  (`--refresh` flag on plan/apply). The `provider.Client.Read` + `state` + the
  no-op/create-decision plumbing already exist.
- Tests: unit tests with a stub client — drift (read differs from stored → plan
  uses read), out-of-band deletion (read empty → create), and `--refresh=false`
  (no read, stored used); a gated AWS check is optional.
- Closes beans-q3ku; completes epic ft9v.
