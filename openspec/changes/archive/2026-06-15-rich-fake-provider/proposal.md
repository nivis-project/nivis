# Proposal: rich-fake-provider

## Why
The codec now handles collections and nested objects (change codec-collections),
but every existing fake provider (alpha/beta/gamma) is **string-only**, so
nothing exercises the new codec end-to-end through a real spawn → plan → apply.
The codec's unit tests cover it in isolation, but the full pipeline
(executor encodes config → provider → decodes new state) has never carried a
list/map/object value. This change adds a fake provider with rich-typed
attributes and an integration test, closing that gap hermetically — and
de-risking real-provider (AWS) plan/apply before credentials are involved.

## What changes
- A new fake tfprotov6 provider `provider-delta` with a resource `delta_thing`
  exercising the collection/object types:
  - inputs: `tags` (map(string)), `ports` (list(number)), `label` (string, opt)
  - computed outputs (unknown at plan, known at apply): `id` (string),
    `endpoints` (list(string)), `meta` (object({ region=string, count=number }))
  Outputs are deterministic functions of the inputs + the seedable counter, like
  the other fakes.
- Because the existing `fakeprovider` base is string-typed
  (`Apply` uses `map[string]string`), `provider-delta` builds its values with a
  small richer apply path that uses `internal/tfcodec` to convert plain Go
  values to/from `tftypes` — which also exercises the codec from the provider
  side. The existing string-only fakes and base are left unchanged.
- An integration test driving the real `provider-delta` binary through the plugin
  manager: plan (collection/object computed attrs unknown) → apply → assert the
  decoded outputs equal the deterministic collection/object values; and that a
  map/list *input* is encoded and seen by the provider.

## Non-goals
- Changing the existing alpha/beta/gamma fakes or the `fakeprovider` base
  signature (kept string-only to avoid churn; delta is additive).
- SDKv2 nested *blocks* (beans-9qrj) — that is the schema-model/codegen side, not
  the value codec; delta uses nested-object *attributes*, which the codec handles.
- The real AWS plan (separate, next).

## Impact
- New: `cmd/provider-delta` and a small rich-typed fake helper (or an extension
  in a new `internal/fakeproviderx`), plus an integration test under the plugin
  or e2e tests.
- Gives a hermetic, deterministic end-to-end proof that list/map/object values
  survive the full encode → provider → decode round trip — the assurance the AWS
  plan will rely on.
