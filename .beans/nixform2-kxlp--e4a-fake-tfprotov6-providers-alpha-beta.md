---
# nixform2-kxlp
title: E4a Fake tfprotov6 providers (alpha, beta)
status: completed
type: epic
priority: high
tags:
    - critical-path
created_at: 2026-06-15T09:02:40Z
updated_at: 2026-06-15T14:22:06Z
parent: nixform2-hj4w
blocked_by:
    - nixform2-znh8
---

Two in-repo providers with computed outputs; hermetic test substrate. Build early. Spec in docs/TESTING.md. OpenSpec changes: fake-providers.



## Summary of Changes
OpenSpec change `fake-providers` implemented and archived as
`2026-06-15-fake-providers`. Two hermetic, deterministic tfprotov6 providers
plus a shared package:

- `internal/fakeprovider/` — shared package: Server implements all 22
  ProviderServer RPCs (meaningful: schema/validate/plan/apply/read/upgrade;
  rest return unimplemented diagnostics). Resource abstraction + generic
  plan(computed->unknown)/apply(compute known) logic. Seedable per-process
  counter (TERRAE_NIVIS_FAKE_COUNTER, default 0). DynamicValue<->tftypes marshalling.
- `cmd/provider-alpha` — alpha_token: label(opt) -> computed id="alpha-N",
  value="alpha:LABEL:N".
- `cmd/provider-beta` — beta_record: from(req) -> computed endpoint="beta://FROM".
- `internal/fakeprovider/conformance_test.go` — 6 in-process tests: schema,
  unknown-at-plan/known-at-apply with exact values, label-absent, counter
  increment, env seed, beta required-input diagnostic. Hermetic (pass under
  ambient TERRAE_NIVIS_FAKE_COUNTER).

Dependency: terraform-plugin-go v0.31.0 (fetched via module proxy).
Tests: `go test ./...` + `go vet ./...` pass; `go build ./cmd/...` -> binaries
that correctly require the go-plugin handshake. IR conformance suite still 7/7.
The exact output derivations here are the contract the E4b headline e2e asserts.
Unblocks E3 (executor drives these providers).



## Update (M2 / collections)
Added provider-delta (internal/fakeproviderx) — a rich-typed fake with map/list
inputs and list(string)/object computed outputs — and an integration test
proving collections + nested objects round-trip through the real
encode->provider->decode pipeline (OpenSpec rich-fake-provider). De-risks the
real AWS plan hermetically.
