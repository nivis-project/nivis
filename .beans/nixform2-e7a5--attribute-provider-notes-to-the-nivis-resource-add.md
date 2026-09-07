---
# nixform2-e7a5
title: Attribute provider notes to the Nivis resource address
status: todo
type: task
priority: normal
tags:
    - discovered
created_at: 2026-09-07T19:14:11Z
updated_at: 2026-09-07T19:14:11Z
parent: nixform2-kovh
---

Follow-up to `nixform2-ceoh` / OpenSpec change `readable-provider-log-lines`, which shipped readable provider notes and deliberately deferred address attribution. See that change's `design.md`, Decision 5.

## The idea

A provider's log entries carry `tf_resource_type` (`aws_amplify_app`) but never a Nivis address (`aws.aws_amplify_app.site`), so a note today names the type and attribute path:

```
provider note aws_amplify_app.description: unable to require attribute replacement
```

Because plan/apply are **sequential** (no goroutines in `internal/phase/driver.go` or `internal/apply`), the executor always knows which resource's RPC is in flight. Publishing that to `internal/providerlog` would let a note name the real address:

```
provider note aws.aws_amplify_app.site (description): unable to require attribute replacement
```

From there the bean's "optionally" item becomes reachable: annotate the change list itself (`! aws.aws_amplify_app.site  provider note: ...`) instead of printing notes beside it.

## Why it was deferred

- It needs the driver to push state into the logging path — cross-package coupling that wants its own design, rather than being smuggled into a rendering change.
- It only pays off once notes are readable at all, which is what shipped.
- The fallback (`tf_resource_type` + `tf_attribute_path`) already tells you which resource type and attribute a note is about.

## Acceptance criteria

- A note names the Nivis resource address when the executor knows it, and falls back to the provider's `tf_resource_type` when it does not (a note emitted outside a resource RPC, e.g. during schema fetch).
- The correlation is safe if apply ever becomes concurrent: either it is explicitly scoped per in-flight RPC, or the concurrency assumption is asserted in a test that fails when it stops holding.
- Covered by an e2e over `provider-zeta` asserting the address appears.
