# Proposal: codec-collections

## Why
The value codec (both the v6 `internal/tfvalue` and the v5
`internal/provider/v5`) only encodes/decodes scalar attributes
(string/number/bool) — `goToValue`/`valueToGo` error on anything else
(beans-guxs). That was fine for the scalar-only fake providers, but it blocks
real providers: AWS and most real resources use lists, sets, maps, and nested
objects pervasively. Configuring or planning any non-trivial real resource fails
at the codec the moment a collection/object attribute is encoded. This change
extends both codecs to handle those types, the prerequisite for a real
credentialed AWS plan.

## What changes
- Extend `goToValue` (Go value -> tftypes.Value) in both codecs to handle
  `tftypes.List`, `Set`, `Tuple` (from a `[]interface{}`), and `Map`, `Object`
  (from a `map[string]interface{}`), recursing into element/attribute types.
  Unknown/null elements are handled.
- Extend `valueToGo` (tftypes.Value -> Go) in both codecs symmetrically:
  list/set/tuple -> `[]interface{}`, map/object -> `map[string]interface{}`,
  recursing; unknown nested values are dropped from output (as today for scalars).
- Keep behavior identical for scalars and for null/unknown handling. Factor the
  shared recursion so the v5 and v6 codecs stay in lockstep (the wire format is
  identical; only the DynamicValue/Schema protobuf types differ).

## Non-goals
- SDKv2 nested *blocks* in codegen (beans-9qrj) — that is the schema-model side;
  this change is the value codec. (Once schemas expose nested object types, this
  codec can already encode them.)
- DynamicPseudoType / exotic tftypes beyond list/set/map/tuple/object — error
  clearly if encountered, as today.
- The real AWS plan itself — separate, follows this change.

## Impact
- Changed: `internal/tfvalue/value.go`, `internal/provider/v5/tfvalue5.go`.
- New tests over nested fixtures (list/set/map/object, mixed, null/unknown).
- Unblocks real-provider configure/plan/apply for resources with collection and
  object attributes (i.e. essentially all real resources). Closes beans-guxs.
