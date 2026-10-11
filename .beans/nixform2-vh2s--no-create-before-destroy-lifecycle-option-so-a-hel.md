---
# nixform2-vh2s
title: No create-before-destroy lifecycle option, so a held reference cannot be replaced
status: todo
type: feature
priority: normal
tags:
    - discovered
created_at: 2026-10-11T10:02:38Z
updated_at: 2026-10-11T10:02:38Z
---

Discovered while exploring nixform2-nwnf on 2026-10-11.

`ir.Lifecycle` carries `preventDestroy` and `ignoreChanges`. There is no
`createBeforeDestroy`, so a resource can only ever be replaced by destroying it
first.

## Why it is wanted

Replacing a resource that something else still references fails, because the
destroy half runs while the reference is live. A planning fixpoint
(nixform2-nwnf) fixes this for dependents whose reference is force-new, by
putting them in the replacement set and destroying the whole set in reverse
order. It structurally cannot fix a dependent that merely HOLDS the reference
without being force-new: that dependent is never replaced, so it is never
destroyed, so it is still holding the object when the destroy runs.

Terraform has the same wall and its answer is `create_before_destroy`. It is the
standard remedy for exactly the three-layer image topology in nixform2-nwnf
(artefact, cloud-side conversion, machine).

## Why it is not a substitute for the ordering work

It is opt-in per resource and it cannot always be used: two objects must be able
to coexist, which fails for anything with a unique name or identifier, and it
costs double for the overlap. Terraform makes it opt-in for these reasons.

## Fits the phased model well

A deferred destroy at the end of a run is natural in a loop that already runs to
a fixpoint: create the replacement, let later phases repoint dependents, destroy
the old object last.

## Known design points

- Terraform propagates create-before-destroy to dependencies automatically, and
  not doing so is a known source of confusing failures.
- Crash safety: a run interrupted between the create and the deferred destroy
  must not leak the old object silently. nixform2-sw6c establishes the invariant
  this has to honour.
