# Tasks: nested-block-errors

## 1. Codec error messages
- [x] 1.1 In `internal/tfcodec/tfcodec.go`, in `sliceToValue` (List/Set) and
      `tupleToValue`, when `raw` is a `map[string]interface{}`, return an
      actionable error: the block is list-nested, wrap the attrset in a
      one-element list `[ { ... } ]`. Keep the generic `got %T` message for any
      other non-slice type.
- [x] 1.2 In `mapToValue` (Object/Map), when `raw` is a `[]interface{}`, return an
      actionable error: the block takes a single attrset `{ ... }`, not a list.
      Keep the generic `expected object, got %T` for other non-map types.
- [x] 1.3 Confirm the offending attribute name still reaches the top (the
      per-key `["k"]: %w` wrap is unchanged), so the message reads e.g.
      `["disk_container"]: this is a list-nested block; wrap ...`.

## 2. Tests (table-driven, hermetic)
- [x] 2.1 In `internal/tfcodec/tfcodec_test.go`: a `List[Object{...}]` fed a
      `map` returns an error mentioning the one-element-list fix (assert on the
      guidance substring, not the exact full string).
- [x] 2.2 A `Set[Object{...}]` fed a `map` likewise; a `Tuple` fed a `map`
      likewise.
- [x] 2.3 An `Object{...}` (and a `Map`) fed a `[]interface{}` returns the
      single-attrset guidance error.
- [x] 2.4 Valid cases unchanged: a `List[Object]` fed a one-element slice, and an
      `Object` fed a map, both still convert successfully to the same value
      (regression guard).

## 3. Gate
- [x] 3.1 `gofmt`, `go build ./...`, `go test ./...` green.
- [x] 3.2 `openspec validate nested-block-errors` passes.
- [x] 3.3 Archive the change; update beans-krwc (the error half done; note the
      codegen/Nix-validation half remains, deferred).
