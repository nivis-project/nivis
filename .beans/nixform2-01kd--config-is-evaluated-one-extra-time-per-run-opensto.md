---
# nixform2-01kd
title: Config is evaluated one extra time per run (openStore discards its graph)
status: completed
type: bug
priority: normal
tags:
    - discovered
created_at: 2026-09-11T13:58:34Z
updated_at: 2026-09-11T14:57:07Z
---

Discovered while exploring nixform2-7gdt (improve console output).

`nivis apply` runs a full `nix eval` of the configuration twice before applying
anything, with identical inputs:

    applyCmd.RunE
    ├─ openStore(ctx, stdout)                      cmd/nivis/main.go:186
    │   └─ graphFn(ctx) -> phase0Graph(ctx)        cmd/nivis/main.go:166
    │       ├─ newLedger()        fresh ledger + resolved vars
    │       ├─ evaluator().Eval() ==> nix eval #1
    │       └─ ir.IngestIR()
    │   The whole ingested graph is discarded; only g.Backend is read.
    │
    └─ d.Run(ctx)                                  internal/phase/driver.go:131
        └─ phase 0: d.Eval.Eval(ctx, d.Ledger) ==> nix eval #2

Both evals have identical inputs (same flake ref, same attr, same resolved
vars, empty ledger at Phase 0), so eval #2 repeats eval #1's work exactly.

`destroy` and `refresh` do the same in the other order: they call
`phase0Graph` for the graph, then `openStore` calls `phase0Graph` again purely
to read the backend.

Cost: on a configuration where eval takes ~10s, that is ~10s of dead air per
run doing no useful work. It is the cheapest latency win available.

Fix direction: memoize the phase-0 graph per process, or restructure so
`openStore` receives an already-ingested graph rather than evaluating for
itself.

Why this should land BEFORE the console-output work: the event/observer
refactor will emit an "evaluating configuration" event, which makes this bug
directly visible to the user. Shipping the new output first means shipping
output that narrates a bug.


---

**Not closed by the `stream-run-progress` OpenSpec change.** This is an
independent engine fix that should land BEFORE it. `beans list -S
"stream-run-progress"` matches this bean because that change is discussed
above; the match is a sequencing reference, not an implementation link.


---

## Summary of Changes

OpenSpec change: `evaluate-config-once-per-run` (tinychange schema).

A run now evaluates the configuration once per distinct ledger instead of once
per consumer. Two caches in `cmd/nivis/evalcache.go`:

- `evalCache` — a `phase.NixEvaluator` that memoizes the raw IR JSON keyed on the
  marshalled ledger, which is exactly what is handed to Nix. This is what makes
  the phase loop's phase 0 free once backend discovery has already evaluated.
- `configGraph` — memoizes the ingested phase-0 graph for the consumers that want
  a graph (`openStore`, `destroy`, `refresh`, `state migrate`).

Both cache the error as well as the value, so a failed evaluation is not retried
once per consumer.

Call sites routed through `configGraph`: `openStore`, `statemigrate`, and — the
trap — `destroyCmd`/`refreshCmd`, which called `phase0Graph` DIRECTLY, bypassing
the `graphFn` seam. Fixing only `openStore` would have left both still
double-evaluating.

`resetRunCache()` added and wired into the `withGraph` test helper on both sides.
Production never calls it (one command per process), but the tests share a
process: without the reset, seven `statemigrate` tests failed because a graph
memoized by an earlier test meant their swapped `graphFn` was never consulted.

Beyond the latency saving, this is a consistency fix: `nix eval` runs `--impure`,
so two evaluations within one run could legitimately disagree. A run now sees one
snapshot.

Tests added (`cmd/nivis/evalcache_test.go`), all count-based because the results
are identical either way and only latency differs:
- one evaluation across backend discovery and the phase loop's phase 0
- `configGraph` memoized, returning the same snapshot
- a failed evaluation cached, not retried per consumer
- later phases still evaluate; a repeated phase-0 ledger does not
- ledger outputs are part of the key, not just the phase number
- both failure paths unchanged: a state subcommand with no evaluable config still
  falls back to local, while a consumer that needs the config still sees the error
- `TestPhase0LedgersAreIdenticalAcrossConsumers` guards the invariant the sharing
  rests on, since a divergence there would silently restore the duplicate
  evaluation with no test failing

Verified with `go test -count=2 -shuffle=on ./cmd/nivis/...` for isolation.

Discovered while shipping: `nixform2-p72l` — `tests/check-docs-gate.sh` globs
`*/proposal.md`, so a `tinychange` (which has none) passes the docs gate
vacuously. This change's docs decision ("none") is recorded in its `tasks.md`
regardless.
