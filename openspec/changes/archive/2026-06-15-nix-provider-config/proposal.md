# Proposal: nix-provider-config

## Why
A provider's config already reaches `ConfigureProvider` via the IR
`providers.<name>.config` map, and the executor's object-type construction
already handles nested blocks (assume_role, default_tags, endpoints, …). But the
Nix frontend has **no ergonomic, validated way to express provider-level
settings**, and — more importantly — `toIR` passes `providers` through *raw*
(`inherit providers`): it never runs provider config through the ledger-`resolve`
or the `encode` pass that resources get. Consequences today:

- A Nix author writes a bare attr tree for `providers.aws.config` with no
  constructor, no shape guidance, and no error if `source` is missing.
- Provider config **cannot** contain `__ref`/`__derived` leaves, even though the
  IR contract says it "may contain refs" — because `toIR` doesn't resolve or
  encode them. So a provider can't be configured from another resource's output.
- The AWS example must take region/credentials from the environment
  (`AWS_PROFILE`/`AWS_REGION`) because there is no clean way to put `region` in
  the config — making the example non-self-contained and the docs carry a
  caveat.

This change closes the Nix-side gap: an ergonomic `mkProvider` constructor and a
`toIR` that resolves + encodes provider config exactly like resource config, so
`providers.aws = mkProvider { source = …; config = { region = "eu-central-1";
default_tags = { tags = {…}; }; }; }` flows into `Configure`.

## What changes
- **`nix/lib`: add `mkProvider { source, config ? {} }`** returning the IR
  provider shape `{ inherit source config; }`, with a named error if `source` is
  missing or not a string. Nested blocks are plain nested attrs/lists in
  `config` (the executor's ObjectType construction already maps them). Export it
  from `nix/lib/default.nix`. Bare attrsets (`{ source; config; }`) remain
  accepted by `toIR` for backward compatibility — `mkProvider` is the ergonomic,
  validated front door, not a new requirement on callers.
- **`toIR`: resolve + encode provider config.** Run each provider's `config`
  through `resolve ledger` then `encode` (the same two passes resource config
  gets), instead of `inherit providers`. This makes `__ref`/`__derived` leaves in
  provider config work — they resolve against the ledger each phase and encode to
  wire shape — and is a no-op for plain values. `source` passes through
  unchanged.
- **AWS example uses Nix-side region.** `nix/example/aws.nix` declares
  `region` (and optionally `profile`) in `mkProvider`'s config, so the example no
  longer *requires* `AWS_REGION` in the environment. Credentials still come from
  the environment by default (we don't hard-code secrets in Nix), but region and
  any non-secret settings now live in the config. README + getting-started
  updated to reflect that provider config lives in Nix.

## Non-goals
- A typed/schema-checked provider config (validating `region` against the AWS
  provider schema in Nix) — config is still a raw attr tree the provider
  validates at `Configure` time. `mkProvider` validates only `source`.
- Putting **secrets** (access keys) in Nix — left to the environment / SDK
  default chain on purpose; only non-secret settings (region, profile,
  default_tags, …) are the ergonomic target.
- Executor changes — none. The executor already passes the config map to
  `Configure` and handles nested blocks; this is a frontend + serialization
  change.

## Impact
- New: `nix/lib/mkProvider.nix`, exported as `terraeNivis.mkProvider`.
- Changed: `nix/lib/toIR.nix` (resolve+encode provider config),
  `nix/lib/default.nix` (export), `nix/example/aws.nix` (Nix-side region),
  README + `docs/GETTING-STARTED.md` (provider config in Nix).
- Tests: a Nix property test (provider config resolves a `__ref`/`__derived` and
  encodes; `mkProvider` errors without `source`); IR conformance unchanged
  (still valid IR); a Go test asserting a provider config map (incl. a nested
  block) reaches `Configure`.
- Closes beans-prj4.
