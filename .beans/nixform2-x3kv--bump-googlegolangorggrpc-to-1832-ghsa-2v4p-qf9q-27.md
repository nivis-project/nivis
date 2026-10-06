---
# nixform2-x3kv
title: Bump google.golang.org/grpc to 1.83.2 (GHSA-2v4p-qf9q-27wj)
status: completed
type: task
priority: low
tags:
    - discovered
    - security
created_at: 2026-10-06T20:08:16Z
updated_at: 2026-10-06T20:30:16Z
---

Dependabot alert 5: https://github.com/nivis-project/nivis/security/dependabot/5

- Advisory: GHSA-2v4p-qf9q-27wj, severity high
- Package: `google.golang.org/grpc`, currently v1.83.1 in `go.mod`
- Vulnerable range: >= 1.83.0, < 1.83.2; first patched: 1.83.2

The crash is in gRPC-Go xDS servers, reached when a request arrives without
both the `:authority` and `Host` headers. Nivis runs no xDS server; grpc is
used for the go-plugin channel to provider binaries. Real impact here is nil,
so this is a hygiene bump to clear the alert, not a fix for an exploitable
path.

Work: `go get google.golang.org/grpc@v1.83.2 && go mod tidy`, then run the
test suite.


OpenSpec change: `bump-grpc-1-83-2` (schema: tinychange, `skip_specs: true`).
