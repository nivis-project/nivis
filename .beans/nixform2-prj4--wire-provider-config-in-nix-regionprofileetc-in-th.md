---
# nixform2-prj4
title: Wire provider config in Nix (region/profile/etc. in the config, not just env)
status: todo
type: feature
priority: normal
created_at: 2026-06-15T14:53:46Z
updated_at: 2026-06-15T14:53:46Z
parent: nixform2-2vc3
---

Today a provider's config reaches Configure via the IR providers.<id>.config map, but the Nix lib has no ergonomic way to express provider-level settings (region, profile, assume_role, default_tags, endpoints, ...). For real use a Nix author should write e.g. providers.aws = { region = "eu-central-1"; default_tags = {...}; } and have it flow into ConfigureProvider. Currently AWS region/creds come only from the environment (AWS_PROFILE/AWS_REGION) via the SDK default chain. 
Work: extend the Nix lib / flake interface to declare provider config (incl. nested blocks like assume_role/default_tags) and serialize it into the IR providers.<id>.config; the executor already passes that map to Configure and the object-type construction already handles nested blocks. Add a property/e2e test. Discovered 2026-06-15 while doing the real AWS plan/apply.
