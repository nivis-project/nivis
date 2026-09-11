---
# nixform2-8btx
title: Provider warning diagnostics are discarded
status: todo
type: bug
priority: normal
created_at: 2026-09-10T21:02:40Z
updated_at: 2026-09-11T13:15:14Z
parent: nixform2-kovh
---

Provider warning diagnostics are normalised and then dropped, so a provider that
explains exactly what is wrong is never heard.

`normDiags` maps every non-ERROR diagnostic to `provider.SeverityWarning`
(`internal/provider/v6/v6.go:421`, `internal/provider/v5/v5.go:414`). Nothing
ever reads that severity again — `grep -rn SeverityWarning` over the repo
returns three hits: those two assignments and the const declaration in
`internal/provider/provider.go:17`. `DiagError` filters to `SeverityError`
only, so warnings from Plan/Apply/Read/ReadDataSource/Destroy are discarded on
every code path.

`--provider-log-level` does not cover this. That flag surfaces the spawned
provider's hclog stream; protocol diagnostics are a separate channel, so today
there is no setting that makes a warning visible.

## How it showed up

Applying `030_vaultwarden_hetzner` in nivis-demos against
hetznercloud/hcloud v1.68.0 failed with an opaque Hetzner 422:

    invalid input in fields 'assignee_id', 'location'
    Field: assignee_id - either assignee_id or location must be provided
    Field: location    - either assignee_id or location must be provided

The config set `datacenter`, which is a real attribute still present in the
v1.68.0 schema, so nothing rejected it locally. The provider had already said
what was wrong, as a warning diagnostic:

    The 'datacenter' attribute is marked for removal since 'v1.67.0',
    you must use the 'location' attribute instead.
    (https://docs.hetzner.cloud/changelog#2026-07-01-removing-datacenters)

nivis dropped it. Diagnosing a one-line deprecation took reading the schema out
of the provider binary with `strings`. Any provider deprecation reaches users
this way: as a downstream API error with no link back to the cause.

## Scope

- [ ] Carry warning-severity diagnostics out of Plan, Apply, Read,
      ReadDataSource and Destroy alongside the result, rather than only
      converting errors via `DiagError`
- [ ] Surface them as notes attributed to the resource id, in the same shape as
      the existing `provider error <type>:` line
- [ ] Decide the de-duplication rule — a per-resource deprecation warning
      repeats on every phase and every refresh, and must not drown the output
- [ ] Test with a fake provider that returns a warning on Plan and on Apply, and
      assert the warning reaches the console while the run still succeeds
