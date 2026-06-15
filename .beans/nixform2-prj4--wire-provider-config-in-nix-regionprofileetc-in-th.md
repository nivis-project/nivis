---
# nixform2-prj4
title: Wire provider config in Nix (region/profile/etc. in the config, not just env)
status: completed
type: feature
priority: normal
created_at: 2026-06-15T14:53:46Z
updated_at: 2026-06-15T15:55:48Z
parent: nixform2-2vc3
---

Today a provider's config reaches Configure via the IR providers.<id>.config map, but the Nix lib has no ergonomic way to express provider-level settings (region, profile, assume_role, default_tags, endpoints, ...). For real use a Nix author should write e.g. providers.aws = { region = "eu-central-1"; default_tags = {...}; } and have it flow into ConfigureProvider. Currently AWS region/creds come only from the environment (AWS_PROFILE/AWS_REGION) via the SDK default chain. 
Work: extend the Nix lib / flake interface to declare provider config (incl. nested blocks like assume_role/default_tags) and serialize it into the IR providers.<id>.config; the executor already passes that map to Configure and the object-type construction already handles nested blocks. Add a property/e2e test. Discovered 2026-06-15 while doing the real AWS plan/apply.


---
Resolved via OpenSpec change nix-provider-config (archived 2026-06-15-nix-provider-config).
- Added mkProvider { source, config ? {} } to the Nix lib (validated front door; errors on missing/empty source).
- toIR now resolves+encodes each provider's config with the SAME two passes as resource config, so __ref/__derived in provider config resolve each phase and plain values pass through; source is verbatim.
- nix/example/aws.nix declares region + default_tags in Nix via mkProvider; only credentials come from the environment.
- Tests: Nix properties P6 (provider config resolves+encodes) and P7 (mkProvider validates source); Go round-trip proving a provider config map incl. a nested block (default_tags) survives encode->decode into the ConfigureProvider value. README + getting-started updated.
