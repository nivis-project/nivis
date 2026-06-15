---
# nixform2-0g0v
title: gen.Fetch calls GetProviderSchema N+1 times (catastrophic for large providers)
status: completed
type: bug
priority: high
tags:
    - discovered
created_at: 2026-06-15T13:41:12Z
updated_at: 2026-06-15T13:45:45Z
parent: nixform2-dwqg
---

Found running nixform-gen against the REAL AWS provider (6.50.0). internal/gen.Fetch calls client.ListResourceTypes (1 full GetProviderSchema) THEN client.GetSchema per resource type (another full GetProviderSchema EACH). AWS has ~1400 resource types, and its GetProviderSchema response is multi-MB, so this is ~1400 full-schema round trips — it did not finish in minutes. 
Root cause: provider.Client.GetSchema re-fetches the entire provider schema every call (see internal/provider/v6/v6.go and v5/v5.go: each GetSchema does a fresh GetProviderSchema RPC). Fine for 1-resource fakes; pathological for real providers. 
Fix options: (a) add a provider.Client method that returns ALL resource schemas in one call (GetAllSchemas), and have gen.Fetch use it; or (b) cache the GetProviderSchema response in the backend so repeated GetSchema calls are served from memory. (b) is the smaller change and also speeds plan/apply (which call SchemaFor per resource). Recommend (b) plus a gen path that lists+fetches from one cached response.



## Done
Fixed in OpenSpec large-provider-readiness: backends cache GetProviderSchema
(sync.Once); N GetSchema calls -> 1 RPC (test proves it). AWS codegen went from
not-finishing to one fetch + 1672 constructors.
