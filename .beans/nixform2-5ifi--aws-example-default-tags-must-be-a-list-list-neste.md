---
# nixform2-5ifi
title: 'AWS example: default_tags must be a list (list-nested block)'
status: completed
type: bug
priority: normal
tags:
    - discovered
created_at: 2026-06-15T16:58:46Z
updated_at: 2026-06-15T17:02:37Z
---

nix/example/aws.nix declares default_tags = { tags = {...}; } (a bare attrset), but the AWS provider's default_tags is a LIST-nested block (tftypes.List[Object[...]]). tn apply fails at ConfigureProvider: 'attr "default_tags": expected array ..., got map[string]interface{}'. Introduced by the prj4 example addition, which added default_tags without testing apply against a list-nested block (the earlier live verification predated it). Fix: default_tags = [ { tags = {...}; } ]. Discovered 2026-06-15 while verifying the AWS S3 tutorial (beans-807d). The toIR/encode path already supports list config; this is purely the example value.
