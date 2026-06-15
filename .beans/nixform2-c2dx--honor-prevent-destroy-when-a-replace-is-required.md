---
# nixform2-c2dx
title: Honor prevent_destroy when a replace is required
status: completed
type: feature
priority: normal
tags:
    - discovered
created_at: 2026-06-15T17:50:08Z
updated_at: 2026-06-15T20:31:42Z
parent: nixform2-ft9v
---

Once resource-update-replace lands, a force-new change triggers destroy+create. If the resource (or its lifecycle meta-arg) sets prevent_destroy, the executor MUST refuse the replace with a clear error naming the resource, rather than silently destroying it. Non-goal of resource-update-replace; future milestone. The lifecycle meta-args (prevent_destroy/ignore_changes) already exist in the IR/contract — this wires prevent_destroy into the replace decision.


---
Partially done by resource-update-replace: applyOne already refuses a replace with a clear error when lifecycle.preventDestroy is set, with a unit test (TestApplyOneReplaceRefusedByPreventDestroy). Remaining scope for this bean: confirm parity across the destroy command path (destroy.Run already errors on preventDestroy) and add an e2e/integration assertion if wanted; otherwise this can be closed as covered.


---
Status (2026-06-15): the MUST is DONE and unit-tested — applyOne refuses a replace when lifecycle.preventDestroy is set, returning a named error before any Destroy (internal/phase/driver.go:244-245; TestApplyOneReplaceRefusedByPreventDestroy asserts destroyCalls==0); the destroy command path refuses too (internal/destroy/destroy.go:61-62). REMAINING (why this stays open): add a gated AWS e2e — apply a resource with lifecycle.preventDestroy, change a force-new attribute, assert `nivis apply` refuses with the named error and the real resource is NOT destroyed. Optional-but-wanted; that's the only gap.


---
DONE (2026-06-15). The optional gated AWS e2e is added and verified live:
internal/phase/prevent_destroy_aws_test.go (TERRAE_NIVIS_NET_TESTS=1) creates a
real S3 bucket with an explicit name, then re-applies with a renamed bucket
(force-new) + lifecycle.preventDestroy and asserts applyOne REFUSES with the
named error and does not destroy the bucket. Ran against AWS account
076504012268: PASS — "refused as expected: replace of \"aws.aws_s3_bucket.demo\"
requires destroying it, but lifecycle.preventDestroy is set"; bucket untouched,
cleaned up after, no orphan. (Implementation + unit test landed earlier in
resource-update-replace.) c2dx fully covered.
