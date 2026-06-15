# Proposal: schema-codegen

## Why
To reach "all providers with zero per-provider work" (DESIGN D2), nixform needs
to turn any provider's schema into typed Nix constructors automatically. This
change builds that generic pipeline: spawn a provider, fetch its schema, map the
schema to a Nix type model, and emit `<provider>/<type>.nix` constructors —
packaged as a flake app. It is breadth, off the critical path (DESIGN D5), built
after the round trip was proven.

The type mapping is the generic, schema-driven core and is validated against
synthetic schema fixtures covering the hard cases (nested objects, sets, maps,
sensitive, optional+computed). The spawn→schema→codegen pipeline is validated
end-to-end against the real fake provider binaries. Both hermetic, no network.

## What changes
- `internal/gen/schema.go`: spawn a provider via the existing plugin manager,
  call `GetProviderSchema`, and produce a normalized in-memory schema model
  (resource type -> attributes with type, role flags). Reuses
  `internal/plugin` + `internal/tfvalue` type parsing.
- `internal/gen/typemap.go`: map a tftype + role flags to a Nix type descriptor
  (string/number/bool/list/set/map/object; required/optional/computed/sensitive).
  Mine Pulumi's mapping (DESIGN D2): computed-only attrs are outputs (no input
  arg), required throw if missing, optional pass through, sensitive flagged.
- `internal/gen/emit.go`: emit a Nix constructor per resource type — a function
  taking the input attrs (required ones `throw` if absent, optional default to
  omitted) and producing `mkResource { provider; type; name; config; }`. Include
  an **override seam** (the generated module accepts an `overrides` arg merged
  last) so generated code is usable, not the final word (Pulumi overlay lesson).
- `cmd/nixform-gen` + a flake app: `nix run .#gen -- --provider <path> --out <dir>`
  spawns the provider and writes the generated `.nix` files.

## Non-goals
- Provider registry download / real providers (AWS, Hetzner) — network-gated,
  out of PoC scope (CLAUDE.md §6). Tracked as beans nixform2-8umq, which names
  AWS + Hetzner as the designated first real providers for that milestone.
- Full fidelity for every exotic schema feature (write-only, deprecated,
  identity schemas) — map the common cases; record gaps rather than guess.
- Data sources / functions — resources only for the PoC codegen.

## Impact
- New: `internal/gen/` (schema model, type map, emitter), `cmd/nixform-gen`, a
  flake `apps.gen`, and tests (synthetic-schema type-map unit tests + an
  end-to-end gen run against the fake binaries). New dep: none beyond existing.
- Generated constructors produce the same IR (`mkResource`) the rest of the
  stack already consumes; no contract change.
