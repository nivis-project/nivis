# Tasks: tfprotov5-client

- [x] 1.1 Vendored `proto/tfplugin5.proto` (go_package -> our internal path);
      extended `proto/generate.sh` to generate v5 + v6. Generated
      `internal/tfplugin5/*.pb.go`; builds clean (NewProviderClient present).
- [x] 1.2 `internal/provider/v5/tfvalue5.go`: v5 value codec (encodeConfig with
      refs->unknown, encodeState, nullState, decodeState, unknownAttrs,
      objectType from a v5 Schema_Block; same tftypes msgpack wire format).
- [x] 1.3 `internal/provider/v5/v5.go`: backend wrapping tfplugin5.ProviderClient
      implementing provider.Client (ListResourceTypes, GetSchema, Plan, Apply,
      Read, Destroy), normalizing v5 diagnostics + TypeKind. (v5 RPC is GetSchema.)
- [x] 1.4 `internal/plugin/manager.go`: VersionedPlugins{5,6}; after dispense,
      branch on c.NegotiatedVersion() to build the v5 or v6 backend.
- [x] 1.5 `cmd/provider-gamma` + `internal/fakeproviderv5`: a tf5server-served
      fake provider (gamma_widget: required `size` -> computed `id`="gamma-N",
      `result`="widget:size:N"), mirroring alpha/beta determinism.
- [x] 1.6 `internal/plugin/v5_test.go`: spawns the REAL gamma binary, asserts the
      manager negotiates v5 and the plan/apply round trip yields the exact
      deterministic outputs through provider.Client.
- [x] 1.7 `go test ./...` + `go vet ./...` pass; nix tests + IR conformance green;
      v6 e2e unchanged (negotiation didn't regress v6).
- [x] 1.8 `openspec validate tfprotov5-client` passes; change-id in epic
      nixform2-2vc3.
