---
# nixform2-854m
title: Building a __build leaf holds the state lock for the whole build
status: todo
type: task
priority: normal
tags:
    - discovered
created_at: 2026-09-08T12:53:25Z
updated_at: 2026-09-08T12:53:25Z
parent: nixform2-kovh
---

Deferred from `nixform2-ebon` / OpenSpec change `realise-build-leaves-from-drv`, design Decision 5.

Now that `nivis apply` actually builds a `__build` derivation, a build happens **inside the state lock**. `withStateLock` wraps the whole run (`cmd/nivis/main.go`), and realising happens per resource inside `applyOne`, so a 30-minute NixOS image build holds the S3 state lock against every teammate and CI job on that state.

## Why it was not fixed in that change

Before it, builds never happened (they failed), so the lock was never held for long. The fix makes the exposure real but does not create the design.

## The option sketched

Realise the leaves that are already resolvable at phase 0 **before** acquiring the lock. Most `__build` leaves do not depend on apply-time outputs; only the fixpoint case does, and that one must stay inside the loop. That needs new machinery to decide which leaves qualify, which is a design in its own right.

## Acceptance criteria

- A run whose builds are all phase-0-resolvable acquires the state lock only after they are built.
- A build that depends on an earlier resource's apply-time output still happens inside the loop, in its phase (the fixpoint property is not lost).
- The lock-hold window is asserted by a test, not just reasoned about.
