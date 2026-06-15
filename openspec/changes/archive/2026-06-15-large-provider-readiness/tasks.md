# Tasks: large-provider-readiness

- [x] 1.1 `internal/provider/v6/v6.go`: cache the GetProviderSchema response
      (sync.Once); GetSchema + ListResourceTypes read from the cache.
- [x] 1.2 `internal/provider/v5/v5.go`: same schema caching for the v5 backend.
- [x] 1.3 `internal/plugin/manager.go`: ClientConfig.Logger = quiet hclog (Warn),
      suppressing provider TRACE/DEBUG flood.
- [x] 1.4 `internal/provider/v6/cache_test.go`: a counting client proves N
      GetSchema + ListResourceTypes => exactly 1 GetProviderSchema RPC; and the
      cached schema returns correct data.
- [x] 1.5 `go test ./...` + `go vet ./...` pass; nix tests + IR conformance green.
- [x] 1.6 Re-ran nixform-gen against the REAL AWS provider (6.50.0): completed in
      one schema fetch with 0-byte stderr, generated 1672 constructors; computed
      outputs correctly modeled as outputs; a generated constructor evaluates with
      the lib. (Nested blocks omitted -> beans-9qrj.)
- [x] 1.7 `openspec validate large-provider-readiness` passes; beans-0g0v and
      beans-ahjm closed.
