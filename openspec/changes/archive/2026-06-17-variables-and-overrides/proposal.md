# Proposal: variables-and-overrides

## Why
Today a Nivis config cannot be parameterised. The `plan` function receives the
outputs ledger, and authors read ad-hoc fields off `ledger.vars` (the EC2 example
reads `ledger.vars.suffix`), but there is no way to *declare* a variable, give it a
type or default, mark it required, or *set* it from outside (CLI, file, env). So
every per-environment difference is hard-coded or hacked through the ledger. This
is the first thing that blocks Nivis from being a daily-driver for real projects
(beans-kym5 / A1, milestone "Road to v1").

## What changes
First-class variables, declared in Nix and resolved by the executor.

- **Declaration (Nix):** a `nivis.mkVars` helper. The author declares each
  variable with an optional `type` and `default`; a variable with no default is
  **required**. `mkVars` validates the *resolved* values it is handed (from
  `ledger.vars`) against the declarations: a required var that is unset throws an
  actionable error naming it; a value of the wrong type throws naming the var and
  the expected type; declared vars with defaults fall back when unset. It returns
  the resolved, typed attrset the config reads (`vars.region`, `vars.suffix`).
- **Injection (executor):** the executor resolves a variable map from these
  sources and injects it as `vars` in the ledger it already passes to `plan` each
  phase, so `mkVars` sees concrete values. Sources, **lowest to highest priority**
  (Terraform's convention, so an explicit CLI flag always wins and a stale env can
  never silently override it):
  1. defaults declared in Nix (handled by `mkVars`, not the executor),
  2. environment: `NIVIS_VAR_<name>`,
  3. `--var-file <path>` (JSON; later files override earlier),
  4. `--var name=value` flags (later flags override earlier).
- **Wire format (IR):** the outputs ledger gains an optional `vars` object
  (`{ "<name>": <value> }`). This is an IR-contract change, so the contract is
  updated in this change (hard gate). `vars` is injected on **every** phase
  (constant across the fixpoint), alongside the accumulating `outputs`.
- **Purity:** values arrive as data in the 0600 ledger file already used for
  outputs, read with `builtins.fromJSON`. No `--impure` env reads inside Nix, no
  new impurity. The executor reads `NIVIS_VAR_*` from its own environment, not Nix.

## Decisions (settled with the maintainer)
- **`mkVars`, not module-system options, for v1.** Resolved values land in
  `ledger.vars`, the same seam a future `options.vars` module layer could also
  write to, so the powerful module approach stays an *additive* future with no
  rework. `mkVars` ships the useful 80% purely and simply now.
- **Terraform precedence** (`defaults < env < var-file < --var`), to avoid the
  stale-env footgun and ease migration for the TF/OpenTofu audience.

## Non-goals
- **Module-system (`options.vars`) declaration.** Deferred; this change keeps it
  possible (same `ledger.vars` seam) but does not build it.
- **Rich type system.** v1 supports a small set (`str`, `int`, `bool`, and a
  permissive `any`); list/attrset/enum typing can come later. `mkVars` validates
  what it supports and passes `any` through.
- **`.auto.tfvars`-style auto-loading** of files by name. v1 takes explicit
  `--var-file`. Auto-discovery can come later.
- **Sensitive variables / secret injection.** Out of scope here; the existing
  sensitive-output channel is unchanged. A later epic can mark vars sensitive.

## Impact
- IR: `docs/IR-CONTRACT.md` and `docs/ir-schema.json` (ledger gains optional
  `vars`); the conformance suite stays green (additive, optional field).
- Nix: `nix/lib/` gains `mkVars` (exported from `default.nix`); property tests in
  `nix/tests/`.
- Go: `internal/ledger` (`Ledger.Vars`), `cmd/nivis` (`--var`, `--var-file`
  flags), a var-resolution unit with precedence + parsing, wired into the phase
  driver so every eval gets `vars`. Table-driven Go tests + an integration test
  against the in-repo fakes proving a `--var` threads through a phase into a
  resolved config.
- Docs: the AWS S3 tutorial gains a short "parameterise with a variable" note;
  README's Nix-library list gains `mkVars`.
