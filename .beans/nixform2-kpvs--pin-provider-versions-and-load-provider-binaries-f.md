---
# nixform2-kpvs
title: Pin provider versions and load provider binaries from the Nix store
status: todo
type: feature
priority: normal
tags:
    - discovered
    - provider
    - reproducibility
created_at: 2026-11-13T16:18:19Z
updated_at: 2026-11-13T16:18:19Z
parent: nixform2-m83a
---

Today `lib.mkProvider` carries only `source` (a registry address) and `config`. At
runtime `internal/registry` resolves the address to the LATEST version, downloads
the zip from the provider's release host, verifies SHA256 against the registry's
SHA256SUMS, and caches the unpacked binary at
`<user cache>/nivis/providers/<host>/<ns>/<name>/<version>/<os>_<arch>/`.

Consequences:
- No version pin: two machines or two days can run different provider releases.
- The binary is not in the Nix store and is not covered by `flake.lock`.
- Execution depends on runtime network access (air-gapped use is impossible).
- The cached artifact is mutable and outside Nix's integrity model.
- `Fetch` resolves versions before checking the cache, so even a warm cache hits
  the registry (`internal/registry/registry.go`).

Goal: make a provider a pinned, content-addressed Nix artifact.

## Scope
- Optional `version` (and platform) on the provider declaration, e.g.
  `lib.mkProvider { source = "..."; version = "6.28.0"; }`.
- A Nix-side fetcher, e.g. `lib.fetchProvider { source; version; platform ? ...; hash; }`,
  that produces a store path containing `terraform-provider-<name>_v<version>`.
  `source` already accepts a store path (`registry/resolver.go` returns existing
  files verbatim), so this may need no IR change.
- Wire the pinned binary into the IR as either the store path in `source` or an
  explicit new field. Prefer the store path to keep `IR-CONTRACT.md` unchanged.
- Lock the version and hash in the consumer's flake: a `nix flake update`-driven
  input, or a generated lock entry, so `flake.lock` records the provider.
- Platform matrix: `linux_amd64`, `darwin_arm64`, etc. Address whether the flake
  pins one platform or all.
- Leverage the Nix binary cache: a pinned provider should substitute from a cache
  instead of downloading from GitHub on every machine.
- Keep the runtime registry path as the unpinned default.

## Acceptance criteria
- A consumer can pin `hashicorp/aws` to an exact version and platform.
- `nix build` yields the provider binary in the Nix store; `nivis` spawns it from
  there with no network.
- The pin is visible in `flake.lock` (or an equivalent generated lock file) and
  changes only on an explicit update.
- Warm-cache runs do not contact the registry.
- The unpinned address path still works.

## Design questions
- Provider version in the IR or resolved entirely in Nix? (IR is frozen.)
- One platform per pin or a per-platform matrix?
- Reuse the C3 mirror/air-gap work and `nivis-registry`, or stay client-side?
- GPG signature verification in addition to SHA256?

## Docs impact
Needs a provider-pinning section (GETTING-STARTED / INSTALL) and likely a new
docs page, plus a note in IR-CONTRACT if the shape changes.
