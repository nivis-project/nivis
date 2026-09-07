---
# nixform2-ih51
title: Automatic local-first bootstrap detection from the phase-0 IR graph
status: todo
type: feature
priority: normal
tags:
    - discovered
created_at: 2026-09-07T16:41:11Z
updated_at: 2026-09-07T16:41:11Z
parent: nixform2-kovh
---

Follow-up to `nixform2-ebx5` / OpenSpec change `state-migrate-between-backends`, which shipped the EXPLICIT bootstrap (`nivis apply --backend=local` then `nivis state migrate --to-remote`) and deliberately deferred the automatic path. See that change's `design.md`, Decision 5.

## The idea

`openStore` already holds the phase-0 IR graph, so it can ask whether a resource *in this configuration* creates the bucket named in `backend.bucket`:

```go
for _, id := range g.Order {
    r := g.Nodes[id].Resource
    if r.Type == "aws_s3_bucket" && r.Config["bucket"] == backend.bucket { ... }
}
```

That distinguishes "the bucket is missing because I am about to create it" from "the bucket name is a typo" without guessing from `NoSuchBucket` vs `AccessDenied`. With it, a single `nivis apply` could run local-first and flush the document into the bucket on success — the three documented steps, sequenced by the tool.

## Why it was deferred

- The explicit path had to exist first; automatic is just those steps sequenced.
- The detection is unavailable when the bucket name is a computed unknown at phase 0, or when the bucket is created through a wrapper whose type is not literally `aws_s3_bucket` — so the explicit path stays the fallback either way.
- Shipping the manual sequence first let the bootstrap be documented and covered by an e2e test before any inference was layered on.

## Acceptance criteria

- Detection is a pure function over the IR graph, unit-tested including the unknown-bucket-name and non-matching-type cases.
- A single `apply` bootstraps a self-managed bucket: local-first, then the document ends up in the bucket, with a loud notice explaining what happened.
- A missing bucket that this configuration does NOT create still fails with today's actionable error.
- The local file remains a crash-safe journal: an interrupted run leaves a complete local document and a message naming `nivis state migrate --to-remote`.
