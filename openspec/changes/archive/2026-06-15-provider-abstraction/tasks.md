# Tasks: provider-abstraction

- [x] 1.1 `internal/provider/provider.go`: the `Client` interface
      (ListResourceTypes, GetSchema, Plan, Apply, Read, Destroy) + normalized
      types (ResourceSchema with role-flag Attrs + opaque Raw, Diagnostic,
      request/result structs over map[string]interface{}), and DiagError helper.
- [x] 1.2 `internal/provider/v6/v6.go`: a backend wrapping a
      `tfplugin6.ProviderClient` implementing `provider.Client` — encode config
      (refs->unknown), Plan/Apply/Read/Destroy, decode state, normalize
      diagnostics, list types, coarse TypeKind per attr. Reuses internal/tfvalue.
- [x] 1.3 `internal/plugin/manager.go`: `Client` returns `provider.Client`
      (constructs the v6 backend); pooling/Close unchanged.
- [x] 1.4 Refactored callers to the interface: internal/plan (now thin),
      internal/apply, internal/destroy, internal/refresh, internal/gen/schema
      (Fetch discovers types via ListResourceTypes), internal/phase/driver.
      Removed direct tfplugin6/tftypes use from these (verified clean).
- [x] 1.5 Updated test manager helpers (relMgr/relativeManager) + codegen e2e to
      the new return type / Fetch signature; assertions unchanged.
- [x] 1.6 `go test ./...` + `go vet ./...` pass UNCHANGED; nix tests + IR
      conformance green. (Refactor is behavior-preserving.)
- [x] 1.7 `openspec validate provider-abstraction` passes; change-id in epic
      nixform2-2vc3.
