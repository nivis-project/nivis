---
# nixform2-9qrj
title: Codegen omits SDKv2 nested blocks (block_types), only flat attributes
status: completed
type: feature
priority: high
tags:
    - discovered
created_at: 2026-06-15T13:44:47Z
updated_at: 2026-06-15T14:46:14Z
parent: nixform2-dwqg
---

Found generating against real AWS. The schema model + codegen only handle a resource block's flat ATTRIBUTES (Schema_Block.Attributes). SDKv2 providers (AWS, and most v5 providers) also express sub-resources as nested BLOCKS (Schema_Block.BlockTypes / nested_block), e.g. aws_instance's root_block_device, ebs_block_device, network_interface. These are omitted from generated constructors, so a generated AWS constructor is incomplete for resources that use blocks. 
Work: walk Schema_Block.BlockTypes in internal/gen schema parsing (and the v5/v6 backends' schema->objType for the codec), modeling nesting modes (single/list/set/ map) as nested object types. Pairs with beans-guxs (the value codec must then encode those nested objects). Not needed for flat resources (e.g. aws_s3_bucket basics); needed for full AWS fidelity. Generated 1672 AWS constructors successfully otherwise; flat attributes + computed-output detection work correctly.



## Now a real blocker (2026-06-15)
Confirmed via real AWS: nested blocks aren't just a codegen nicety — the AWS
PROVIDER CONFIG itself has 5 block_types (assume_role, assume_role_with_web_identity,
default_tags, endpoints, ignore_tags) on top of 30 attributes. ConfigureProvider
fails with 'an object with 35 attributes is required (30 given)' because our
ObjectType (built from flat Schema_Block.Attributes only) omits block_types. So
nested-block support in the object-type construction is REQUIRED to configure (and
thus plan) AWS at all — raised to high priority. Minimal fix for configure: include
each block_type as a list/set/map(object(...)) attribute per its nesting mode and
send null. Full fix: same in codegen schema model + value codec already handles
nested objects (beans-guxs done).



## Done (type construction)
ObjectType (v6 tfvalue) and objectType (v5) now include nested block_types as
list/set/map(object(...)) per nesting mode, recursing; v6 also handles attr
NestedType. This made real AWS ConfigureProvider succeed (35-attr provider config
with 5 blocks). NOTE: codegen EMISSION of nested blocks into generated .nix
constructors is still flat-only — if richer generated constructors are wanted,
that's a separate codegen-side follow-up. Done via OpenSpec provider-configure.
