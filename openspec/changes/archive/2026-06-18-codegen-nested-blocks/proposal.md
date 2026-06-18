# Proposal: codegen-nested-blocks

## Why
`nivis gen` generates a typed Nix constructor per resource type, but only from a
resource's **flat attributes**: nested blocks (typed in the provider schema's
`Block.BlockTypes` with a nesting mode of single/list/set/map) are absent from the
generated constructor. So a user hand-writes blocks like `default_tags`,
`disk_container`, `ingress`, and `ebs_block_device`, and has to **guess
list-vs-single**. Guessing wrong is the trap `krwc` documented: it fails at apply.
The error half (an actionable message) shipped; this is the **prevention** half
(beans-p4uz gap 1). It also satisfies **A5** (beans-n2rg): once the generated
constructor lists every argument including blocks, with types and nesting, it *is*
the per-provider argument reference, so "Terraform docs to Nivis" becomes "read
the generated constructor."

## What changes
- **The version-neutral provider schema carries nested blocks.** `provider.Client`'s
  schema result (`ResourceSchema`) gains the resource's nested blocks: each a name,
  a nesting mode (single / list / set / map), and its inner attributes (recursing
  for nested-in-nested blocks). The v6 and v5 backends populate this from
  `Schema.Block.BlockTypes` (which the value codec already reads), so codegen sees
  blocks without parsing protobuf itself.
- **Codegen models and emits nested blocks.** The gen `Resource` model gains the
  blocks; `Emit` renders each block as a **typed argument with the shape that
  matches its nesting**, so the correct shape is the obvious one:
  - a **list/set**-nested block is an argument defaulting to `[]`, with a doc
    comment naming it list-nested and showing the `[ { ... } ]` shape and its inner
    fields;
  - a **single**-nested block is an argument defaulting to `null`, a plain attrset;
  - a **map**-nested block is an argument defaulting to `{}` (a map of attrsets).
  Each block's inner attributes are listed in the doc comment, so the generated
  file is self-documenting. (Not per-block builder helpers: a typed arg plus a
  comment is the lighter generator and the more intuitive call site.)
- **A5, minimally:** a short docs note that the generated constructor is the
  per-provider argument reference (run `nivis gen`, read the `.nix`). No separate
  Markdown reference site or hand-maintained pages.

## Decisions (settled with the maintainer)
- **Typed arg + nesting doc comment**, not generated per-block builder helpers
  (heavier generator machinery; the comment-and-shape approach is lighter and more
  intuitive).
- **A5 = the generated constructor is the reference**, documented in one paragraph.
  No Markdown doc-site pages (too early; a second artifact that would rot).
- **Deferred: p4uz gap 2** (Nix-side schema validation at eval time) until there is
  demand; the actionable executor error already covers the failure case.

## Non-goals
- Nix-side / eval-time schema validation (p4uz gap 2, deferred).
- A generated Markdown reference site (deferred).
- Per-block builder-helper functions.
- Datasource-schema codegen (`mkData` constructors); resources only here.

## Impact
- `internal/provider`: `ResourceSchema` gains a nested-blocks field (a recursive
  block type: name, nesting, attrs, sub-blocks); `internal/provider/v6` and
  `internal/provider/v5` populate it from `Block.BlockTypes`.
- `internal/gen`: the model gains blocks; `schema.go` maps them; `emit.go` renders
  block args + doc comments with the right per-nesting shape.
- A fake provider gains a nested block so the path is tested hermetically (the
  fakeprovider already supports nested blocks in its schema for the value codec).
- Tests: `internal/gen` (a schema with a list-nested and a single-nested block
  emits the right arg shapes and comments); provider backend tests that blocks are
  surfaced.
- Docs: a short "the generated constructor is your provider reference" note.

Docs impact: new paragraph/section; an A5 note (generated constructor as the
argument reference) plus a line that generated constructors now include nested
blocks. No new document: it extends the already-documented `nivis gen` flow (per
docs/DOCS-GATE.md).
