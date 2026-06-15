---
# nixform2-guxs
title: Value codec handles only string/number/bool (no list/set/map/object)
status: todo
type: feature
priority: high
tags:
    - discovered
created_at: 2026-06-15T13:30:56Z
updated_at: 2026-06-15T13:30:56Z
parent: nixform2-2vc3
---

Both value codecs — internal/tfvalue (v6) and internal/provider/v5/tfvalue5 (v5) — only encode/decode scalar attributes (string, number, bool). goToValue and valueToGo error with 'unsupported attribute type' on list/set/map/object. This was fine for the PoC fakes (alpha/beta/gamma are scalar-only) but BLOCKS real providers: AWS (and most real resources) use collections and nested objects pervasively. 
Impact by operation:   - GetProviderSchema / nixform-gen schema fetch: WORKS (no value encoding).   - Configure provider / Plan / Apply of any non-trivial real resource: FAILS at     the codec the moment a collection/object attribute is encoded or decoded. 
Work: extend goToValue/valueToGo (both codecs) to handle tftypes List/Set/Map/Tuple/ Object recursively, mapping to Go slices/maps; mirror in EncodeConfig/DecodeState/ EncodeState. The gen NixType model already represents these kinds, so the codec is the gap. Also handle null/unknown nested values. Add table tests over nested fixtures. 
Surfaced 2026-06-15 while scoping real AWS testing. Schema-fetch AWS validation does NOT need this; real AWS plan/apply DOES.
