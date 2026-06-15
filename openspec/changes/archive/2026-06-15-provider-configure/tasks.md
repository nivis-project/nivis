# Tasks: provider-configure

- [x] 1.1 `internal/provider`: added `Configure(ctx, config map[string]interface{})
      error` to the Client interface (config keyed by provider-config attr name;
      absent -> null, so the provider applies its own defaults).
- [x] 1.2 `internal/provider/v6`: Configure builds the provider config object from
      the cached schema's Provider block, encodes (tfcodec), calls
      ConfigureProvider, surfaces diagnostics.
- [x] 1.3 `internal/provider/v5`: Configure via the v5 Configure RPC (no
      PrepareProviderConfig needed — AWS configured fine without it).
- [x] 1.4 `internal/plugin/manager.go`: Client(identity, path, config) configures
      once on first spawn (pooled). Empty config is a no-op for the fakes.
- [x] 1.5 Pass IR `providers.<id>.config` into Client at the driver/destroy/
      refresh call sites; gen passes {}; tests updated.
- [x] 1.6 (nested blocks) `tfvalue.ObjectType` (v6) + `objectType` (v5) now
      include block_types as list/set/map(object) per nesting mode (and v6 attr
      NestedType), recursing — required so the provider config object conforms
      (AWS: 35 attrs incl. 5 blocks). beans-9qrj.
- [x] 1.7 Fixed PriorState: Plan/Apply now send a NULL-encoded value of the
      resource type, not an empty DynamicValue — SDKv2 providers panic (EOF)
      decoding empty msgpack. (The fakes tolerated it; real AWS did not.)
- [x] 1.8 Gated real-AWS test `internal/plugin/aws_test.go` (NIXFORM_NET_TESTS=1 +
      AWS creds): registry-fetch hashicorp/aws -> negotiate v5 -> Configure ->
      Plan aws_s3_bucket -> planned state, 26 known-after-apply attrs, no error,
      no resource created. RAN IT: PASS.
- [x] 1.9 `go test ./...` + `go vet ./...` pass (gated test skips by default);
      nix tests + IR conformance green; gofmt clean. `openspec validate` passes.
