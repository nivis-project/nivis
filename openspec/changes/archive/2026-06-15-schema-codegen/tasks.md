# Tasks: schema-codegen

- [x] 1.1 `internal/gen/model.go`: normalized schema model — `Resource{Type,
      Attrs}`, `Attr{Name, Type, Required, Optional, Computed, Sensitive}` with
      `IsInput()`, and a `NixType` descriptor (kind + element/attr children).
- [x] 1.2 `internal/gen/schema.go`: `Fetch(ctx, mgr, identity, path) ([]Resource)`
      — spawn via internal/plugin, GetProviderSchema, walk ResourceSchemas,
      parse each attr's tftype (ParseJSONType) + NestedType into the model.
- [x] 1.3 `internal/gen/typemap.go`: `mapType` covering scalars, list/set/map
      (recursive element), object (recursive attrs); roles from flags;
      computed-only -> output (not input) via `Attr.IsInput`.
- [x] 1.4 `internal/gen/emit.go`: a Nix constructor `{ name, <inputs> ? null,
      overrides ? {} }:` that throws a friendly named error on missing required,
      conditionally merges optional inputs, omits computed-only, calls mkResource,
      and merges `overrides` last. Deterministic (sorted attrs).
- [x] 1.5 `cmd/nixform-gen/main.go`: cobra CLI `--provider`/`--identity`/`--out`;
      writes `<out>/<provider>/<type>.nix`. (Flake `apps.gen` packaging is
      network-gated — needs nixpkgs buildGoModule; tracked as beans nixform2-28sn.
      Today: `go run ./cmd/nixform-gen`.)
- [x] 1.6 Unit tests `internal/gen/gen_test.go`: typemap over synthetic schemas
      (scalars, list/set/map, nested object), attr roles, emit structure
      (required-throw, optional conditional, computed-not-input, override seam,
      determinism).
- [x] 1.7 e2e `tests/e2e/codegen_test.go`: run Fetch+Emit against the real
      provider-alpha binary; the generated `alpha_token.nix` imports with the lib
      and yields a valid mkResource (id + config correct). Skips if nix absent.
- [x] 1.8 `go test ./...` + `go vet ./...` pass; nix tests + IR conformance green.
- [x] 1.9 `openspec validate schema-codegen` passes; change-id in beans E2;
      registry download (beans-8umq, AWS+Hetzner) + flake-app packaging
      (beans-28sn) remain network-gated.
