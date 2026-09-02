---
# nixform2-ceoh
title: Render provider log lines as readable warnings (not raw hclog) in plan/apply
status: todo
type: task
priority: normal
created_at: 2026-08-31T22:04:31Z
updated_at: 2026-09-02T10:14:33Z
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
