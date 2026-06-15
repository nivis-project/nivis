# nixform

A Nix-native infrastructure tool where Terraform/OpenTofu **provider resources
are first-class Nix values**. A thin Go executor speaks the Terraform plugin
protocol (`tfprotov6`) directly to **unmodified provider binaries** — Nix is the
configuration frontend, Go is pure orchestration.

The headline capability — the reason this project exists — is the **round trip**:
a provider-created resource returns computed values (an IP, an ID, a generated
secret) back into Nix, which **re-evaluates** to produce dependent configuration,
repeating to a fixpoint. This is proven end to end across two providers with
unknown values originating on both sides (see the demo below).

## How it works (one paragraph)

Nix evaluates your configuration to a JSON **IR** (`docs/IR-CONTRACT.md`). Values
that aren't known until apply-time are emitted as typed placeholders — a `__ref`
(a direct reference to another resource's output) or a `__derived` (a value Nix
*computed* from an output, e.g. a string built from an IP). The Go executor
ingests the IR, spawns the relevant provider binaries, drives
`GetProviderSchema`/`PlanResourceChange`/`ApplyResourceChange`, and collects the
real outputs into an **outputs ledger**. It then **re-evaluates Nix** with the
ledger injected, so placeholders resolve to concrete values; the new IR may
unlock more resources. This loop repeats to a **fixpoint** (no new value
resolves). Because each Nix-mediated (`__derived`) hop needs its own
re-evaluation, deep chains take more than two phases — the loop generalizes to
N phases. See `DESIGN.md` for why this (not an `Output<T>` promise model) is the
honest, Nix-shaped approach.

## Try the demo (offline, no network, no credentials)

Everything runs against in-repo **fake providers** that speak `tfprotov6` — no
registry, no cloud account. You need Go 1.22+ and Nix.

```sh
# Build the fake providers and the CLI.
go build -o bin/provider-alpha ./cmd/provider-alpha
go build -o bin/provider-beta  ./cmd/provider-beta
go build -o bin/nixform        ./cmd/nixform

# Plan and apply the headline topology (resolves across 3 phases to a fixpoint).
./bin/nixform plan
./bin/nixform apply

# Inspect state — note C's label is a value Nix derived from BOTH providers.
./bin/nixform state list
./bin/nixform state show alpha.alpha_token.C

# Reconcile (no changes) and tear down (reverse dependency order: C, B, A).
./bin/nixform refresh
./bin/nixform destroy
```

Generate typed Nix constructors from a provider's schema:

```sh
go run ./cmd/nixform-gen -- --provider ./bin/provider-alpha --out ./generated
cat ./generated/alpha/alpha_token.nix
```

See `docs/GETTING-STARTED.md` for a guided walkthrough.

## Layout

- `nix/lib/` — the Nix library: `mkResource`, references (`__ref`/`__derived`),
  `toIR`, `count`/`for_each` expansion, and a module system (`evalModules`).
- `flake.nix` — exposes `nixform.plan` (a function of the outputs ledger → IR).
- `internal/` — the Go executor: `ir` (ingest/validate), `graph` (DAG, TF→TF
  resolution), `state`, `plugin` (spawn + go-plugin v6 handshake), `plan`/`apply`,
  `phase` (the fixpoint loop), `destroy`/`refresh`, `gen` (schema codegen).
- `cmd/` — `nixform` (the CLI), `nixform-gen` (codegen), and the fake providers.
- `tests/`, `nix/tests/` — Go unit/e2e tests, the IR conformance checker, and Nix
  property tests.

## Stable contracts

These are versioned interfaces — change the spec before the shape:

- **The IR**: `docs/IR-CONTRACT.md` (prose) + `docs/ir-schema.json` (the
  normative JSON Schema). Both the Nix `toIR` producer and the Go `IngestIR`
  consumer validate against it; `tests/ir-conformance/check.py` is the executable
  conformance suite.
- **The flake interface**: `nixform.plan = ledger → IR`, evaluated each phase
  with the outputs ledger injected.

## Testing

```sh
go test ./...                      # Go unit + e2e (uses the fake providers)
bash tests/run-nix-tests.sh        # Nix property tests + IR conformance
python3 tests/ir-conformance/check.py test   # IR schema/referential conformance
```

The milestone exit test is `tests/e2e` (`TestHeadlineRoundTrip`): two providers,
unknowns on both sides, ≥3 phases to fixpoint, a Nix consumer reading both.

---

This repository was built autonomously following a spec-driven process: see
`DESIGN.md` (architecture decisions), `ROADMAP.md` (epics), `CLAUDE.md` (the
builder's instructions), and `openspec/` (the per-change specs). Real provider
support via the OpenTofu registry is network-gated and out of the current PoC
scope; the codegen and executor are validated hermetically against the fakes.
