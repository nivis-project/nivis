---
# nixform2-9qrj
title: Codegen omits SDKv2 nested blocks (block_types), only flat attributes
status: todo
type: feature
priority: normal
tags:
    - discovered
created_at: 2026-06-15T13:44:47Z
updated_at: 2026-06-15T13:44:47Z
parent: nixform2-dwqg
---

Found generating against real AWS. The schema model + codegen only handle a resource block's flat ATTRIBUTES (Schema_Block.Attributes). SDKv2 providers (AWS, and most v5 providers) also express sub-resources as nested BLOCKS (Schema_Block.BlockTypes / nested_block), e.g. aws_instance's root_block_device, ebs_block_device, network_interface. These are omitted from generated constructors, so a generated AWS constructor is incomplete for resources that use blocks. 
Work: walk Schema_Block.BlockTypes in internal/gen schema parsing (and the v5/v6 backends' schema->objType for the codec), modeling nesting modes (single/list/set/ map) as nested object types. Pairs with beans-guxs (the value codec must then encode those nested objects). Not needed for flat resources (e.g. aws_s3_bucket basics); needed for full AWS fidelity. Generated 1672 AWS constructors successfully otherwise; flat attributes + computed-output detection work correctly.
