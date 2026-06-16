---
# nixform2-6e6i
title: 'A2: Datasources'
status: todo
type: epic
priority: high
tags:
    - roadmap
created_at: 2026-06-16T13:37:59Z
updated_at: 2026-06-16T13:37:59Z
parent: nixform2-zdj0
---

Read existing infrastructure (an AMI by filter, a VPC, an AZ) and feed it into resources, the way every other IaC tool can.

ROADMAP Phase A2. GROUND TRUTH: the provider protocol method ReadDataSource exists on our fakes/real providers but the executor never drives it, and there is no datasource constructor in nix/lib.

## Scope
- Nix-lib constructor (mkData or similar) for a datasource node.
- Executor drives ReadDataSource per phase, like any other node; outputs feed refs.
- IR-CONTRACT.md addition for the datasource node shape and its outputs (OpenSpec change to the contract FIRST).
- A datasource-serving fake provider for hermetic tests.

Datasource reads participate in the phased fixpoint (a data lookup may depend on an earlier resource's output).
