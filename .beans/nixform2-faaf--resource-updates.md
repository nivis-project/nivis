---
# nixform2-faaf
title: resource updates
status: todo
type: bug
priority: normal
created_at: 2026-06-15T17:31:40Z
updated_at: 2026-06-15T17:42:22Z
parent: nixform2-ft9v
---

I did the s3 tutorial and after creating the bucket i added the bucket property
to give it a choosen name. This didn't update the existing bucket, but created
a new bucket.

- Is this a bug or did I catch a not implemented, but essential feature of terrae nivis ?
- How hard is it to implement this in a generic global manner fopr all future supported resources?


---
ANSWER (investigated 2026-06-15, read-only):

Neither a one-off bug nor a regression — it's an UNIMPLEMENTED lifecycle feature, by PoC scope. The executor is structurally CREATE-ONLY:

- Both Plan and Apply hardcode a null PriorState to the provider (internal/provider/v5/v5.go:146 and :177; v6 identical). provider.PlanRequest has no PriorState field at all (internal/provider/provider.go); plan.Plan() never reads the state store (internal/plan/plan.go; internal/phase/driver.go:143).
- So on a SECOND apply with changed config, the provider sees PriorState=null and plans a CREATE. The AWS provider duly creates a NEW bucket for the new `bucket` name; the old bucket is never destroyed (orphaned in AWS), and state.Set is a blind UPSERT keyed by id (internal/state/state.go:138) so the old state entry is overwritten.
- RequiresReplace from the plan response is also discarded (provider.PlanResult has no such field), so even force-new attributes aren't detected.
- Contrast: destroy (internal/destroy/destroy.go:54) and refresh (internal/refresh/refresh.go:29) DO read prior state from the store — the read-back pattern exists; plan/apply just don't use it.

Resource identity IS matched by id (<provider>.<type>.<name>) — and the id (aws.aws_s3_bucket.demo) didn't change, only config.bucket did. The bug the user hit is that a changed force-new attribute should REPLACE (destroy+create); changed normal attributes should UPDATE in place. Today neither happens — every apply is a fresh create.

Why it was never built: the PoC milestone is the round trip (create + read computed values back into Nix across phases), not full lifecycle. Update/replace was simply out of PoC scope (not in docs/ROADMAP.md; lifecycle there only covers prevent_destroy/ignore_changes meta-args).

FIX SCOPE (generic, all resources — not per-resource):
Plan must feed prior state instead of null. Concretely:
1. Plan reads existing state for the id (store.Get) and passes its attrs as PriorState (the read-back plumbing already exists for destroy/refresh).
2. Add PriorState to provider.PlanRequest; v5/v6 backends encode it (EncodeState already exists) instead of priorNull when prior state is present; null only for a genuine create.
3. Read RequiresReplace from the plan response into provider.PlanResult; when set, do destroy-then-create (the destroy path exists); otherwise ApplyResourceChange does an in-place update (PriorState = stored, PlannedState = planned) — which the provider handles generically.
4. apply writes the new attrs (already does) and, on replace, removes the old resource first.

This is GENERIC — it's all at the protocol/state layer (PlanRequest/PlanResult, state.Get, the existing encode/decode + destroy paths), nothing per-resource. The provider decides create/update/replace from (PriorState, ProposedNewState) + its own schema, exactly as Terraform/OpenTofu do. Estimate: a focused epic — one OpenSpec change for plan-with-prior-state + RequiresReplace + replace path, with fake-provider update/replace tests + an AWS apply-twice e2e. Medium, not large, because the read-back, encode/decode, and destroy primitives already exist.


---
Scoped into OpenSpec change resource-update-replace (under epic beans-ft9v), validated. Implementation pending.
