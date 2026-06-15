# Spec: registry

## Purpose
The registry fetches real providers so terrae nivis can drive them by address instead
of a hand-built binary path. It resolves a provider address against the OpenTofu
registry, downloads the platform archive from its release host (GitHub), verifies
it against the published SHA256SUMS before unpacking, and caches the executable.
This is the breadth that, together with multi-protocol support, lets the executor
run the real ecosystem (AWS, Hetzner). Network is used only on a cache miss.
GPG signature verification and private/mirror registries are out of PoC scope.
## Requirements
### Requirement: Resolve a provider address to a download
The registry SHALL resolve a provider address (`<ns>/<name>`, optionally
host-qualified) and platform (os/arch) to a concrete download URL, a SHASUMS URL,
and a filename, selecting a version (latest by semver unless constrained) via the
OpenTofu registry API.

#### Scenario: resolve to a platform download
- GIVEN the address `hetznercloud/hcloud` and platform `linux/amd64`
- WHEN resolved
- THEN it yields a `download_url`, `shasums_url`, and `filename` for a concrete version.

### Requirement: Verify the download against published checksums
The registry SHALL compute the SHA-256 of the downloaded archive and require it
to equal the checksum listed for the archive's filename in the published
SHA256SUMS. A mismatch SHALL be a hard error and the bytes SHALL NOT be executed.

#### Scenario: matching checksum is accepted
- GIVEN an archive whose SHA-256 matches its SHA256SUMS entry
- WHEN verified
- THEN verification passes.

#### Scenario: tampered archive is rejected
- GIVEN an archive whose bytes do not match the published checksum
- WHEN verified
- THEN it fails with an error naming the filename and the expected vs actual sum, and no binary is produced.

### Requirement: Cache the unpacked provider binary
The registry SHALL cache the verified, unpacked provider executable keyed by
host/namespace/name/version/platform, returning the cached path on a subsequent
request without re-downloading.

#### Scenario: cache hit skips the network
- GIVEN a provider already downloaded and cached
- WHEN it is requested again
- THEN the cached binary path is returned and no network request is made.

### Requirement: Registry-address provider sources
A provider `source` that is a registry address SHALL be resolved, verified, and
cached to a local binary before spawn; a filesystem path SHALL continue to be
used directly.

#### Scenario: filesystem path still works
- GIVEN a provider source that is an existing filesystem path
- WHEN the provider is spawned
- THEN the path is used directly with no registry interaction.

