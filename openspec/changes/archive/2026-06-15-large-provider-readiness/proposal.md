# Proposal: large-provider-readiness

## Why
Running against the real AWS provider (6.50.0) surfaced two bugs the tiny fake
providers could never expose (beans-0g0v, beans-ahjm): the schema is fetched
O(resources) times, and the provider's TRACE logs flood the terminal (~685 MB for
one AWS schema fetch). Both make any real-provider operation impractical. This
change fixes them so large real providers are usable.

## What changes
- **Cache the provider schema per backend**: `provider.Client.GetSchema` and
  `ListResourceTypes` currently each issue a fresh `GetProviderSchema` RPC; for
  AWS that is ~1400 multi-MB round trips. Fetch the full schema once per spawned
  provider and serve `GetSchema`/`ListResourceTypes` from that cached response
  (both v5 and v6 backends). This also speeds plan/apply (which call SchemaFor
  per resource).
- **Suppress provider logs**: configure the go-plugin client with a quiet hclog
  logger (Warn+), so provider TRACE/DEBUG no longer floods stderr. Errors and the
  actual operation output remain visible.

## Non-goals
- The value codec for collections/objects (beans-guxs) — separate change; this
  one is purely about making schema fetch and spawn practical at AWS scale.
- A "get all schemas in one call" addition to the neutral interface — caching the
  backend's GetProviderSchema response achieves the same without an interface
  change.

## Impact
- Changed: `internal/provider/v6` and `internal/provider/v5` (schema cache),
  `internal/plugin/manager.go` (quiet logger). New dep usage: `hashicorp/go-hclog`
  (already in the module graph).
- After this, `nixform-gen` against the real AWS provider completes in one schema
  fetch, and spawning real providers no longer dumps hundreds of MB of logs.
