# Proposal: features-tutorial

## Why
Phase A of "Road to v1" is complete (variables, datasources, colored/phased
output, completion, codegen reference, state ergonomics, outputs), but there is no
single hands-on tutorial a user can run to exercise it all, and writing one
surfaced two real gaps in the shipped features:

1. **A typed `int`/`bool` variable could not be set from the CLI.** `--var` and
   `NIVIS_VAR_*` values are always strings, but `mkVars`' `int` check rejected the
   string `"5"`, so `--var replicas=5` failed for an `int` var.
2. **An output referencing a datasource did not resolve on a standalone `nivis
   output`.** Datasources are not persisted to state, so `ResolveOutputs`
   (which seeds the ledger from state) left a datasource-derived output as an
   unresolved `__ref`.

Both are fixed, and a runnable feature tutorial is added that would have caught
them.

## What changes
- **`mkVars` coerces string inputs to the declared scalar type.** For an `int` or
  `bool` variable, a string value (as the CLI / env always supply) is parsed to
  the declared type (`"5"` -> `5`, `"true"` -> `true`); a value already of the
  right type (e.g. a typed `--var-file` JSON value) passes through; an unparseable
  string throws the existing named, typed error. `str`/`any` are unchanged.
- **`ResolveOutputs` re-reads datasources.** After seeding from state and
  evaluating, it reads the ready datasources (reads are pure/idempotent), adds
  their outputs to the ledger, and re-evaluates, so an output (or consumer)
  referencing a datasource result resolves to a concrete value instead of a
  `__ref`.
- **A hands-on feature tutorial** (`docs/TUTORIAL-FEATURES.md`), against the
  in-repo fake providers (no cloud, no credentials, deterministic), exercising
  variables, a datasource, the round trip across phases, stack outputs, the
  colored phase-grouped plan/apply, completion, and `state pull`/`push`. It is
  backed by a bundled config `nix/example/tutorial.nix` exposed as the flake attr
  `nivis.tutorial`, so the commands in the tutorial run as written.

## Non-goals
- New feature surface; this completes and demonstrates the Phase A features.
- Float/list/attrset variable coercion (only `int`/`bool` string coercion, which
  is what the CLI needs).

## Impact
- Nix: `nix/lib/vars.nix` (string->int/bool coercion in `mkVars`).
- Go: `internal/phase/driver.go` (`ResolveOutputs` re-reads datasources).
- Example/flake: `nix/example/tutorial.nix` + `flake.nix` `nivis.tutorial` attr.
- Tests: nix property P9 extended (string coercion); a Go e2e
  (`TestStackOutputsResolveDatasource`) guarding the datasource-output fix, plus
  the existing outputs e2e.
- Docs: `docs/TUTORIAL-FEATURES.md` (+ docs-site page + SUMMARY entry).

Docs impact: new document, docs/TUTORIAL-FEATURES.md (a hands-on hermetic tour of
the daily-driver features), surfaced on the docs site (TUTORIAL-FEATURES.md
include + SUMMARY entry). A tutorial is a distinct thing a user looks for, so it
gets its own page (per docs/DOCS-GATE.md).
