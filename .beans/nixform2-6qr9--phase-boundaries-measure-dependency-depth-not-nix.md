---
# nixform2-6qr9
title: Phase boundaries measure dependency depth, not Nix round trips
status: in-progress
type: bug
priority: normal
tags:
    - discovered
created_at: 2026-09-11T13:58:53Z
updated_at: 2026-09-14T20:16:02Z
---

Discovered while exploring nixform2-7gdt (improve console output).

## The mismatch

`internal/phase/driver.go:8-12` documents the intent:

> The phase count is driven by Nix-mediated (__derived) dependencies: a derived
> value only becomes concrete after its inputs are in the ledger AND Nix is
> re-evaluated, so each such hop needs its own phase.

The code does not do that. `driver.go:166` computes the resolve ONCE per phase,
against the ledger as it stands at phase start, and never recomputes it inside
the phase:

    res := graph.ResolveTFTF(g, d.Ledger.ToGraphOutputs())   // frozen for the phase
    for _, id := range res.FullyKnown {
        ...
        d.Ledger.Append(id, outs)   // appended, but res is never recomputed
    }

`res.Configs[id]` is likewise resolved once, at phase start.

So if B holds a plain TF->TF ref to A, and A applies in phase 1, B is not in
`res.FullyKnown` for that phase. B waits for phase 2 -- and phase 2 costs a
full `nix eval` even though nothing Nix-mediated happened.

Confirmed by `internal/phase/datasource_integration_test.go:117`, which expects
3 phases for a plain "resource -> datasource -> resource" chain.

    DOCSTRING PROMISES                 CODE DOES
    ------------------                 ---------
    phase boundary = a Nix             phase boundary = one level of
    round trip (__derived hop)         dependency depth, whatever the cause

    A -tf-> B -tf-> C  = 1 phase       A -tf-> B -tf-> C  = 3 phases, 3 evals
    A -derived-> B     = 2 phases      A -derived-> B     = 2 phases (correct)

## Why it matters

1. Performance: a linear 5-deep plain TF chain costs 5 `nix eval`s where 1
   would do.
2. Honesty of the console output: the phased round trip is the project thesis
   and the headline e2e exit criterion ("resolved across >=3 phases"). If the
   output prints phases prominently, it should be reporting Nix round trips,
   not ordinary dependency ordering. Today a reader cannot tell which
   boundaries were real round trips.

## Two directions

(a) Leave the engine, sharpen the reporting. The data exists: `ir.ClassStarToNix`
    is exactly what makes a node Nix-pending. The renderer can label a
    derived-forced boundary differently from a depth-only one.

(b) Fix the engine: re-resolve within a phase after each apply, so a boundary
    occurs only when a `__derived` leaf needs re-eval. Phase count drops, eval
    count drops, and the phase narrative becomes precisely the thesis.

(b) is the better end state but is a semantics change: it would move the
`AppliedPhases != 3` assertions in driver_test.go:114,
integration_test.go:60, datasource_integration_test.go:116 and
tests/e2e/headline_test.go:91.

## Open question before anyone touches this

Check against docs/DESIGN.md D3 first. The snapshot-per-phase design may be
deliberate (determinism, the ledger-save boundary, treating an ingested IR as
valid only as a coherent snapshot). Re-resolving in-phase is sound for the
TF->TF case specifically -- the config text does not change -- but the intent
should be confirmed, not assumed.

Deliberately NOT part of the console-output change: this is an engine change
with real semantics.


---

**Not closed by the `stream-run-progress` OpenSpec change.** That change
deliberately excludes phase-boundary semantics: this is an engine change with
real semantics and its own test impact.


---

## Investigation, 2026-09-14 — the open question is settled

The bean asked whether the snapshot-per-phase design might be deliberate, and
said to check `docs/DESIGN.md` D3 before touching anything. Checked. It is not
deliberate: **the code contradicts D3**, which is a decided architecture
invariant, not a docstring.

D3 states the two reference flavours and what each costs, verbatim:

> - **TF→TF:** resource A's output feeds resource B's input. Resolved *inside*
>   the executor during apply; **no re-eval needed**.
> - **\*→Nix:** a Nix expression computes something from an apply-time value ...
>   Requires re-eval with the value injected. **This is what drives phase count.**

So a TF→TF chain must not cost a phase per link. It does today.

### Confirmed empirically, not just by reading

A throwaway test with a plain `__ref` chain and NO `__derived` anywhere:

    A -> B (__ref A.value) -> C (__ref B.value)

    AppliedPhases = 3
      phase 1: alpha.alpha_token.A
      phase 2: alpha.alpha_token.B
      phase 3: alpha.alpha_token.C

Per D3 this is one phase. Each extra phase is an extra full `nix eval`.

### The milestone exit criterion is SAFE

The worry was that fixing this would collapse the headline e2e below its
"≥3 phases" criterion. It does not. `tests/e2e/headline_test.go` contains no
`__derived` literal, but its IR comes from the REAL evaluator against
`nivis.plan` (`nix/example/default.nix`). Evaluated at phase 0, that IR is:

    __derived occurrences: 4
    __ref occurrences    : 3
      alpha.alpha_token.A -> {}
      beta.beta_record.B  -> {"from": {"__derived": {"inputs": ["alpha.alpha_token.A.value"]}}}
      alpha.alpha_token.C -> {"label": {"__derived": {"inputs": ["beta.beta_record.B.endpoint", ...]}}}

Both hops are genuinely Nix-mediated (the config uses `str [...]`, which
concatenates an apply-time value in Nix). The headline still needs 3 phases
after the fix, for the right reason.

### The other AppliedPhases assertions

All three use `__derived` and should be unaffected:

- `internal/phase/integration_test.go:60` — "B from=rec-+A.value (__derived)",
  "C label=B.endpoint::A.value (__derived on both)"
- `internal/phase/driver_test.go:114` — comment: "chain is Nix-mediated"
- `internal/phase/datasource_integration_test.go:116` — its stub builds both
  hops with `derivedOrValue`

To be verified by running them, not assumed.

### Consequence for scope

Smaller and safer than this bean originally feared: direction (b) — fix the
engine — is what D3 requires, and no existing phase-count assertion is expected
to move. Direction (a) (leave the engine, relabel in the renderer) is now off
the table: it would document a behaviour the design rejects.

Extra urgency from v0.7.0: the CLI now prints phase headings prominently, so a
user reads "Phase 2" as a Nix round trip when it may be plain dependency depth.
The mismatch moved from a docstring to the screen.

OpenSpec change: `resolve-tftf-within-a-phase`.
