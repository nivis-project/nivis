# Tasks: codegen-nested-blocks

## 1. Provider schema carries nested blocks
- [x] 1.1 `internal/provider`: `ResourceSchema` gains `Blocks []NestedBlock`, with
      `NestedBlock { Name; Nesting (single|list|set|map); Attrs []Attr; Blocks
      []NestedBlock }` (recursive).
- [x] 1.2 `internal/provider/v6`: in GetSchema, walk `sch.GetBlock().GetBlockTypes()`
      and populate Blocks (map the tfplugin6 nesting consts; recurse into inner
      blocks). The value codec already reads these, so mirror its mapping.
- [x] 1.3 `internal/provider/v5`: same over tfplugin5.
- [x] 1.4 Backend tests: a fake/provider schema with a list-nested and a
      single-nested block surfaces both with correct nesting + inner attrs.

## 2. Codegen models + emits blocks
- [x] 2.1 `internal/gen` model: `Resource` gains `Blocks []Block`
      (Name, Nesting, Attrs, sub-Blocks); `schema.go` maps provider NestedBlock
      to the gen model.
- [x] 2.2 `internal/gen/emit.go`: emit each block as a constructor argument with
      the per-nesting default (`[]` list/set, `null` single, `{}` map), include it
      in the built config when non-empty/non-null, and write a doc comment naming
      the nesting and the block's inner attribute names. Deterministic ordering.
- [x] 2.3 gen tests: a resource with a list-nested block emits `ingress ? []` +
      a "list-nested" comment; a single-nested block emits `x ? null` + an attrset
      comment; required inner-attr docs present. Existing flat-attr emission
      unchanged.

## 3. Fake provider with a nested block (hermetic test substrate)
- [x] 3.1 Ensure at least one fake provider's schema declares a nested block (the
      fakeprovider schema already supports nested blocks for the codec; add one to
      a resource used by the gen test, or use a schema fixture).

## 4. Docs (A5, minimal; docs-coverage gate)
- [x] 4.1 A short note (getting-started or INSTALL) that the generated constructor
      is the per-provider argument reference: `nivis gen` lists every argument
      (incl. nested blocks with their nesting) and the computed outputs. No
      Markdown reference site. No em dashes.

## 5. Gate
- [x] 5.1 `gofmt`, `go build ./...`, `go test ./...` green.
- [x] 5.2 `bash tests/run-nix-tests.sh` green (a generated constructor with a block
      still evaluates).
- [x] 5.3 `bash tests/check-docs-ssot.sh` green (docs touched).
- [x] 5.4 `openspec validate codegen-nested-blocks --strict`; archive; close
      beans-n2rg (A5) and beans-p4uz gap 1 (note gap 2 deferred).
