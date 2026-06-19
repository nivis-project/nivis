# Getting started with Nivis

This is a self-contained Nivis project that teaches the core idea from scratch:
the **round trip**, where provider-created values feed back into Nix, which
re-evaluates to resolve dependent configuration across phases. It runs entirely
offline against the in-repo fake providers (no cloud, no credentials).

## Before you start

You need `nivis` and the fake providers on your PATH. The one-liner that gives you
both (and `nivistutor`) in a throwaway shell:

```sh
nix shell github:wearetechnative/nivis#nivis github:wearetechnative/nivis#tutor
```

The config references the providers by bare name (`source = "provider-alpha"`),
so `nivis` finds them on PATH.

## What this builds

`config.nix` describes three resources wired so each hop crosses the Nix boundary:

```
alpha_token.A            (alpha)   -- no inputs; A.value computed at apply
   name = "rec-" + A.value         -- a value Nix derives from A's output
beta_record.B  (beta)    from=name -- B.endpoint computed at apply
   final = B.endpoint + "::" + A.value   -- derived from BOTH providers
alpha_token.C  (alpha)   label=final
```

Because `name` and `final` are values Nix computes *from* provider outputs, they
cannot be known until those outputs exist and Nix is re-evaluated. That is what
forces more than one phase.

## Run it

From this directory:

```sh
nivis plan
nivis apply
```

Apply runs in three phases, not one: phase 1 applies A; re-evaluating with
`A.value` known unlocks B; re-evaluating with `B.endpoint` known unlocks C. The
loop halts at a fixpoint once nothing new resolves.

Inspect the round trip:

```sh
nivis state list
nivis state show alpha.alpha_token.C
```

`C.label` is a string Nix built from both `B.endpoint` (beta) and `A.value`
(alpha), concrete only after both providers applied and Nix re-evaluated.

Read the stack outputs:

```sh
nivis output            # all named outputs
nivis output combined   # a single value
nivis output --json     # machine-readable, for CI or another stack
```

## Next

```sh
nivis refresh   # reconciles state via the providers; no changes here
nivis destroy   # tears down in reverse dependency order
```

Try `nivis gen --provider provider-alpha --out ./generated` to turn a provider's
schema into typed Nix constructors. When you are ready for the daily-driver
features (variables, datasources, outputs in one graph), scaffold the
**features** tutorial with `nivistutor`.
