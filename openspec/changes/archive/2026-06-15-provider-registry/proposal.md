# Proposal: provider-registry

## Why
With the executor speaking both protocols (changes 1–2), the last piece for real
providers is fetching them. This change resolves a provider address against the
OpenTofu registry, downloads the platform binary from its release host (GitHub),
**verifies it against the published SHA256SUMS**, and caches the unpacked binary
— so `nixform` can run real providers (AWS, Hetzner) by address instead of a
hand-built path. Network reachability was re-verified: the registry API and
GitHub releases are reachable from this environment.

## What changes
- `internal/registry`: 
  - **Resolve**: query `registry.opentofu.org/v1/providers/<ns>/<name>/versions`,
    pick a version (latest by semver, or a requested constraint), then
    `.../<version>/download/<os>/<arch>` for the `download_url`, `shasums_url`,
    and `filename`.
  - **Download + verify**: fetch the zip and the SHA256SUMS, compute the zip's
    SHA-256, and require it to match the sum listed for `filename`. A mismatch is
    a hard error (no execution of unverified bytes).
  - **Cache + unpack**: store under a cache dir keyed by
    `<host>/<ns>/<name>/<version>/<os>_<arch>`; unzip the provider executable;
    return its path. A cache hit skips the network.
- `nixform` integration: a provider `source` of the form
  `registry.opentofu.org/<ns>/<name>` (or `<ns>/<name>`) is resolved+cached to a
  local binary before spawn; a filesystem path still works as before.
- Tests: unit tests for resolution parsing, SHA-256 verification (good + tampered),
  and cache hit/miss using local fixtures (hermetic). A network-gated test
  (skipped unless `NIXFORM_NET_TESTS=1`) that really resolves+downloads+verifies
  Hetzner hcloud.

## Non-goals
- GPG signature verification of the SHASUMS file (checksum verification only for
  the PoC; record GPG as a follow-up). 
- Mirror/auth/private registries; OpenTofu's full provider-installation protocol.
- Live `apply` against a real cloud — the real-provider e2e (separate, next) does
  schema fetch + plan only, no resource creation.

## Impact
- New: `internal/registry`, registry resolution in the CLI/manager path. New
  behavior: provider `source` may be a registry address. Network is used only on
  a cache miss and only for allowlisted hosts (registry API + GitHub).
- Closes the download/verify/cache part of beans-8umq. Real providers can now be
  fetched and run; the remaining gap (live cloud apply with credentials) is out
  of scope by design.
