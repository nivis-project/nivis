# Tasks: executor-core

- [x] 1.1 `internal/ir/types.go`: Go types for the IR (Provider, Resource,
      Edge, NixConsumer) and the in-memory model (`ResourceNode`, `RefEdge`,
      `ProviderConfig`, `MetaArgs`). Leaf representation that can hold a value,
      a `Ref`, a `Derived`, or a `SensitiveRef`. (RefEdge carries both LeafPath
      — location in owner config — and TargetPath — path into target output.)
- [x] 1.2 `internal/ir/ingest.go`: `IngestIR([]byte) (*Graph, error)` — unmarshal
      + validate against the contract (schemaVersion, unique ids, edge endpoints
      exist, provider declared, ref/derived targets exist, no count/for_each in
      config or meta). Errors name the offending resource/edge/path.
- [x] 1.3 `internal/ir/classify.go`: classify each ref/derived leaf TF→TF vs
      \*→Nix; walk configs (TF→TF + derived) and consumer values (\*→Nix).
- [x] 1.4 `internal/graph/dag.go`: build DAG from `RefEdge` + `dependsOn` +
      explicit edges; DFS cycle detection (names the cycle); `Ready(done)`
      deterministic ready set. Derived (\*→Nix) leaves create no executor dep.
- [x] 1.5 `internal/graph/resolve.go`: `ResolveTFTF(g, outputs)` substitutes
      known outputs into TF→TF ref leaves (nested map/list paths); returns
      per-resource fully-known vs pending; leaves `__derived`/\*→Nix untouched.
- [x] 1.6 `internal/state/state.go`: `Store` interface + JSON file impl with an
      advisory file lock (syscall.Flock); get/set/list/delete by id; atomic
      write (temp+rename).
- [x] 1.7 Tests (table-driven): ingest valid + each malformed class (named
      errors); classification; DAG readiness + depends_on + cycle; TF→TF
      resolution incl. nested path + derived-stays-pending; state round-trip,
      list/delete, concurrent-writer serialization, partial-state persistence.
- [x] 1.8 `go test ./...` and `go vet ./...` pass. (IR conformance still 7/7.)
- [x] 1.9 `openspec validate executor-core` passes; change-id recorded in beans
      epic E3.
