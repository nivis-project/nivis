---
# nixform2-uu26
title: Executor plugin client needs generated tfplugin6 gRPC stubs (proto codegen)
status: completed
type: feature
priority: high
tags:
    - discovered
created_at: 2026-06-15T09:52:44Z
updated_at: 2026-06-15T10:07:54Z
parent: nixform2-pf2g
---

Discovered while starting E3. terraform-plugin-go ships NO public v6 CLIENT: the gRPC stubs live in tfprotov6/internal/tfplugin6 (un-importable). The .proto IS shipped (tfprotov6/internal/tfplugin6/tfplugin6.proto), so the executor's plugin manager must generate its own client stubs from it.

Verified toolchain (2026-06-15): protoc 34.1 from nixpkgs store (cached); protoc-gen-go + protoc-gen-go-grpc install from the Go proxy (go install OK). go-plugin v1.7.0 (transitive) provides the client harness; handshake constants from tf6server: ProtocolVersion=6, MagicCookieKey=TF_PLUGIN_MAGIC_COOKIE, MagicCookieValue=d602bf8f470bc67ca7faa0386276bbdd4330efaf76d1a219cb4d6991ca9872b2, plugin map key 'provider'.

Plan: this is the 2nd E3 OpenSpec change (plugin-client-plan-apply). The 1st change (executor-core) covers IR ingestion, DAG, ref classification, TF->TF resolution, and JSON state — none need gRPC and are fully testable now. Aligns with DESIGN D2 (spawn unmodified providers, speak the protocol; do not fork/link).



## Summary of Changes
Done in OpenSpec change plugin-client-plan-apply (archived
2026-06-15-plugin-client-plan-apply). Generated tfplugin6 client stubs from the
vendored proto (proto/generate.sh -> internal/tfplugin6), built the go-plugin v6
client manager (internal/plugin), value codec (internal/tfvalue), and plan/apply
engines. Integration test spawns the real fake binaries and proves the
single-provider round trip (handshake + GetSchema + Plan + Apply) with exact
deterministic outputs.
