# Tasks: fake-providers

- [x] 1.1 Add `terraform-plugin-go` dependency; `go mod tidy` clean. Confirm
      `tfprotov6`, `tfprotov6/tf6server`, `tftypes` resolve. (v0.31.0; fetched
      via the module proxy.)
- [x] 1.2 `internal/fakeprovider/base.go`: an embeddable base implementing all
      `tfprotov6.ProviderServer` RPCs with safe unimplemented defaults (empty
      response + diagnostic), so concrete providers override only what they need.
      (Compile-time `_ tfprotov6.ProviderServer = (*Server)(nil)` assertion.)
- [x] 1.3 `internal/fakeprovider/value.go`: helpers to build a resource schema,
      marshal/unmarshal `DynamicValue` <-> `tftypes.Object`, and read the seedable
      per-process counter (`TERRAE_NIVIS_FAKE_COUNTER`, default 0; atomic increment).
- [x] 1.4 `internal/fakeprovider/resource.go`: a `Resource` abstraction holding
      the resource type name, its attribute set, and an apply func; wire generic
      `GetProviderSchema`, `ValidateResourceConfig`, `PlanResourceChange`
      (computed attrs -> unknown), `ApplyResourceChange` (compute known values),
      and `ReadResource` (passthrough) over a set of `Resource`s.
- [x] 1.5 `cmd/provider-alpha/main.go`: define `alpha_token` (label optional;
      computed id/value) with `value="alpha:"+label+":"+n`, `id="alpha-"+n`;
      serve via `tf6server.Serve`.
- [x] 1.6 `cmd/provider-beta/main.go`: define `beta_record` (from required;
      computed endpoint) with `endpoint="beta://"+from`; serve via `tf6server.Serve`.
- [x] 1.7 `internal/fakeprovider/conformance_test.go`: table-driven in-process
      test driving each provider server through GetProviderSchema -> Plan ->
      Apply; asserts unknown-at-plan, known-at-apply, exact derived values,
      counter determinism + env seed, and beta's required-input diagnostic.
      Tests are hermetic (pass under ambient TERRAE_NIVIS_FAKE_COUNTER).
- [x] 1.8 Build both binaries (`go build -o bin/...`); `go vet ./...` and
      `go test ./...` pass. Binaries refuse to run without the go-plugin
      handshake (correct). Built paths: `bin/provider-alpha`, `bin/provider-beta`
      (gitignored build artifacts; executor epic builds them on demand).
- [x] 1.9 `openspec validate fake-providers` passes; change-id recorded in beans
      epic E4a.
