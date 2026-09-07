---
# nixform2-ceoh
title: Render provider log lines as readable warnings (not raw hclog) in plan/apply
status: completed
type: task
priority: normal
created_at: 2026-08-31T22:04:31Z
updated_at: 2026-09-07T19:23:00Z
parent: nixform2-kovh
---

`nivis plan`/`apply` forward the provider's raw hclog diagnostic lines straight
to the user's terminal. They're structured, verbose, and — worst of all — carry
`error="..."` inside a `[WARN]`, which reads as a failure when it isn't.

Real example on every plan/apply of an `aws_amplify_app`:

```
2026-08-31T23:58:27.974+0200 [WARN]  provider.terraform-provider-aws: unable to
require attribute replacement: tf_resource_type=aws_amplify_app
tf_attribute_path=description tf_mux_provider="*schema.GRPCProviderServer"
tf_provider_addr=registry.terraform.io/hashicorp/aws tf_rpc=PlanResourceChange
@caller=.../helper/customdiff/force_new.go:66 @module=sdk.helper_schema
error="ForceNew: No changes for description" tf_req_id=dd296e15-... timestamp=...
```

This is just the AWS SDK's customdiff saying "considered forcing a replace over
`description`, didn't, because nothing changed" — pure noise.

## Problems

1. Unreadable: hclog key/value spew (`tf_req_id`, `@caller`, `tf_mux_provider`,
   ...) interleaved with the change list.
2. Alarming: `error="..."` embedded in a `WARN` looks like a real error to users.
3. Always-on: no level control; benign provider internals print every run.

## Ask

- Parse provider hclog and **pretty-print**: `level  resource: message` (drop
  the `tf_*`/`@caller` cruft, or keep it only under a verbose flag).
- **Level control**: default to hiding provider `WARN`/`DEBUG`; add
  `--log-level` (or honor a `TF_LOG`-style env) to opt back in.
- Optionally, attach a concise annotation/symbol next to the affected resource
  in the change list (e.g. `! aws.aws_amplify_app.site  provider note: ...`)
  instead of free-floating log lines above/among the plan output.

## Acceptance

- A normal `plan`/`apply` shows a clean change list with no raw hclog.
- Provider warnings, when surfaced, are one readable line each and clearly
  distinguished from actual errors.
- Verbose/log-level flag restores full provider logs for debugging.

## Summary of Changes

Shipped as OpenSpec change `readable-provider-log-lines` (archived:
`openspec/changes/archive/2026-09-07-readable-provider-log-lines/` in the `nivis`
store). Commit `f255275e`.

**Before / after**, for the exact entry this bean reported:

```
2026-08-31T23:58:27.974+0200 [WARN]  provider.terraform-provider-aws: unable to
require attribute replacement: tf_resource_type=aws_amplify_app
tf_attribute_path=description tf_mux_provider="*schema.GRPCProviderServer" …
error="ForceNew: No changes for description" tf_req_id=dd296e15-… timestamp=…
      ... × once per resource
```
```
provider note aws_amplify_app.description: unable to require attribute replacement (detail: ForceNew: No changes for description)
      ... and at the end of the run:
provider note aws_amplify_app.description: unable to require attribute replacement (11 further occurrence(s) not shown)
```

**What was built**

- `internal/providerlog`: entry decode, subject derivation
  (`tf_resource_type` narrowed by `tf_attribute_path`, provider identity as
  fallback), one-line rendering with the telemetry dropped, and a `Sink` that
  collapses repeats (fingerprint = subject + message, so a differing `detail`
  does not split a note) and reports counts at end of run. 98% covered.
- `internal/plugin`: the manager hands go-plugin a **JSON-formatting** hclog
  logger whose `Output` is the sink, so hclog is a serializer and every rendering
  decision lives in one place. `WithProviderLog(sink, level)` follows the
  existing `WithResolver` style.
- `--provider-log-level` (`off|error|warn|info|debug|trace`), global, applied to
  every provider-spawning command. `trace` restores the dropped fields.
- `cmd/provider-zeta`: a fake that emits this bean's verbatim entry, on both
  routes the transport distinguishes.

**The three problems, each closed**

1. *Unreadable* — one line, telemetry gone; notes on stderr, change list on
   stdout, so a redirected change list is unmixed.
2. *Alarming* — the `error` field renders as `detail:`, never as an error; the
   e2e asserts the run still succeeds while emitting the note.
3. *Always-on* — `--provider-log-level`, plus collapse so a per-resource note
   costs one line and a count.

**Where the ask was declined, deliberately**

The bean asked to hide provider `WARN` by default. The design (Decision 1)
declines: a level low enough to hide the AWS SDK's benign customdiff chatter also
hides deprecation notices and replacement warnings, permanently and invisibly.
Warnings stay visible and are made bearable by rendering and collapsing instead.
The recorded escalation, if that proves wrong in practice, is a narrow documented
suppression list keyed on `@module` + message — which rots visibly in a file,
unlike a hidden threshold.

**Two facts implementation turned up** (both in the change's design.md, §7,
because the next log-emitting fake will hit them):

- A provider's writes to `os.Stderr` **after** serving begins are discarded:
  go-plugin replaces `os.Stderr` with the plugin stdio channel, delivered to
  `SyncStderr` (`io.Discard`). The transport's log reader reads the process's
  real descriptor — which is where a real provider lands, because it builds its
  logger at init. `provider-zeta` captures the descriptor before serving.
- A `plan` of a stack **not in state never contacts the provider**: `PlanReport`
  only calls it for resources found in state. So a fresh plan emits no notes at
  all; `apply` is the first run that reaches `PlanResourceChange`. The e2e and
  the CLI spec scenarios were corrected to say so.

**Verified**: `internal/providerlog` table tests over golden entries including the
verbatim AWS one; `internal/plugin` tests spawning the real fake; five e2e cases
over the real CLI (readable + not-a-failure, collapse across four resources,
`error` silence and `trace` restoration, unknown level refused, notes-on-stderr).
Coverage moved 67.9% -> 69.6% overall and the floors were ratcheted up.

**Discovered work filed**: `nixform2-e7a5` (attribute notes to the Nivis resource
address, and from there annotate the change list — design Decision 5).
