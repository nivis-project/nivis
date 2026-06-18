---
# nixform2-zdj0
title: 'M1: Road to v1 (a daily-driver for Nix developers)'
status: completed
type: milestone
priority: high
tags:
    - roadmap
created_at: 2026-06-16T13:37:23Z
updated_at: 2026-06-18T15:54:44Z
---

The PoC milestone (nixform2-hj4w) is complete: the round trip is proven, real AWS apply/update/replace/destroy works, codegen and the EC2+NixOS example land. Nivis is experimental/alpha (0.3.x).

This milestone takes Nivis from "the demo works" to "I can run my real infrastructure on this." See docs/ROADMAP.md (rewritten) for the full plan; this milestone covers **Phase A** (daily-driver for Nix developers). Phases B (team-ready) and C (enterprise) are tracked as separate epics/milestones for the longer horizon.

## Definition of done (Phase A)
A Nix developer can manage a real, multi-resource project end to end, day to day, WITHOUT dropping back to Terraform: typed variables/overrides, datasource lookups, a legible (colorised, phase-aware) plan/apply/destroy, shell completion, and per-provider reference docs. Shared remote state and locking are Phase B; this milestone is the single-operator daily-driver.

## Invariants (DESIGN.md)
spawn-not-link providers; Nix is a batch evaluator (phased re-eval to fixpoint, no Output<T>); the IR is frozen (IR-affecting work updates IR-CONTRACT.md via OpenSpec first); tests run against in-repo fakes; nivis.lib stays builtins-only.

## Epics (Phase A)
- A1 Variables and overrides
- A2 Datasources
- A3 Legible plan/apply/destroy output
- A4 Shell completion
- A5 Per-provider reference docs
- A6 State ergonomics


---
## Milestone complete (2026-06-18)
Phase A delivered: A1 variables, A2 datasources, A3 colored/phased output, A4 completion, A5 per-provider reference (generated constructors), A6 state ergonomics, A7 stack outputs, plus the docs-coverage gate (hibe) and the nested-block error fix (krwc). A Nix developer can manage a real multi-resource project end to end without dropping to Terraform. Hands-on tour: docs/TUTORIAL-FEATURES.md. Release notes: docs/releases/road-to-v1.md. (p4uz gap 2, Nix-side eval validation, reparented to the enterprise milestone as deferred.)
