# Tasks: provider-registry

- [x] 1.1 `internal/registry/resolve.go`: parse a provider address; query the
      OpenTofu registry versions API, pick latest by semver; resolve the
      platform download (download_url, shasums_url, filename, version).
- [x] 1.2 `internal/registry/verify.go`: parse SHA256SUMS; `verify` computes
      sha256 and requires equality, erroring (named) on mismatch/missing entry.
- [x] 1.3 `internal/registry/cache.go`: cache dir keyed by
      host/ns/name/version/os_arch; cachedBinary (hit) + storeBinary (unzip the
      terraform-provider-* executable 0755).
- [x] 1.4 `internal/registry/registry.go`: `Client.Fetch` ties resolve+download+
      verify(before unpack)+cache; cache hit skips network. Default cache under
      os.UserCacheDir/nixform/providers.
- [x] 1.5 `internal/registry/resolver.go` + `internal/plugin` Resolver seam:
      manager `.WithResolver`; a non-path, address-shaped source is fetched
      before spawn; a filesystem path passes through. CLI wires the resolver.
- [x] 1.6 Unit tests (hermetic): address parsing, LooksLikeAddress, SHASUMS
      parse + verify (match/tampered/missing), storeBinary unzip + reject, cache
      hit + resolver passthrough.
- [x] 1.7 Network-gated test (NIXFORM_NET_TESTS=1): really resolved + downloaded
      + verified hetznercloud/hcloud; ran it once — PASS. Also probed end-to-end:
      registry fetch -> spawn -> negotiate v5 -> read real schema (29 resource
      types).
- [x] 1.8 `go test ./...` + `go vet ./...` pass (net test skipped by default);
      nix tests + IR conformance green.
- [x] 1.9 `openspec validate provider-registry` passes; change-id in epic
      nixform2-2vc3; beans-8umq download/verify/cache satisfied.
