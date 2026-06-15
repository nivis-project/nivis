<p align="center">
  <img src="docs/assets/banner.png" alt="Terrae Nivis — Infrastructure as Nix Code" width="100%">
</p>

# Terrae Nivis

**Infrastructure as Nix Code.** *(Terrae Nivis — Latin, "lands of snow"; formerly
`nixform`.)*

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
go build -o bin/tn ./cmd/tn

# Plan and apply the headline topology (resolves across 3 phases to a fixpoint).
./bin/tn plan
./bin/tn apply

# Inspect state — note C's label is a value Nix derived from BOTH providers.
./bin/tn state list
./bin/tn state show alpha.alpha_token.C

# Reconcile (no changes) and tear down (reverse dependency order: C, B, A).
./bin/tn refresh
./bin/tn destroy
```

Generate typed Nix constructors from a provider's schema:

```sh
go run ./cmd/tn-gen -- --provider ./bin/provider-alpha --out ./generated
cat ./generated/alpha/alpha_token.nix
```

See `docs/GETTING-STARTED.md` for a guided walkthrough.

## Real providers (AWS)

Terrae Nivis drives **real** Terraform/OpenTofu providers, not just the fakes: it
resolves a provider by address from the OpenTofu registry, downloads and
**checksum-verifies** the binary from its release host, negotiates the plugin
protocol (v5 or v6), configures it, and runs the same plan/apply/destroy cycle.

> ⚠️ **This creates a real resource in your AWS account.** The example below
> creates a single (free-tier) S3 bucket and then destroys it. Provider settings
> like `region` live in the **Nix config** (via `mkProvider`); only credentials
> come from the environment (the AWS SDK default chain) — set `AWS_PROFILE` (or
> `AWS_ACCESS_KEY_ID`/…). First run downloads the ~900&nbsp;MB AWS provider
> (cached after).

```sh
export AWS_PROFILE=your-profile          # credentials only; region is in the Nix config

./bin/tn plan    --attr terraeNivis.aws  # show the planned bucket
./bin/tn apply   --attr terraeNivis.aws  # create a real S3 bucket (AWS-generated name)
./bin/tn state show aws.aws_s3_bucket.demo
./bin/tn destroy --attr terraeNivis.aws  # delete it
```

The `terraeNivis.aws` flake attribute (`nix/example/aws.nix`) declares the
provider with `mkProvider { source = "registry.opentofu.org/hashicorp/aws";
config = { region = "eu-central-1"; default_tags = …; }; }` and one
`aws_s3_bucket` — change the `region`, source, or resource to drive any other
provider/setting/resource the same way. Provider config (including nested blocks
like `default_tags`) flows into the provider's `Configure` call.

## Layout

- `nix/lib/` — the Nix library: `mkResource`, `mkProvider`, references
  (`__ref`/`__derived`), `toIR`, `count`/`for_each` expansion, and a module
  system (`evalModules`).
- `flake.nix` — exposes `terraeNivis.plan` (a function of the outputs ledger → IR).
- `internal/` — the Go executor: `ir` (ingest/validate), `graph` (DAG, TF→TF
  resolution), `state`, `plugin` (spawn + go-plugin v6 handshake), `plan`/`apply`,
  `phase` (the fixpoint loop), `destroy`/`refresh`, `gen` (schema codegen).
- `cmd/` — `tn` (the CLI), `tn-gen` (codegen), and the fake providers.
- `tests/`, `nix/tests/` — Go unit/e2e tests, the IR conformance checker, and Nix
  property tests.

## Stable contracts

These are versioned interfaces — change the spec before the shape:

- **The IR**: `docs/IR-CONTRACT.md` (prose) + `docs/ir-schema.json` (the
  normative JSON Schema). Both the Nix `toIR` producer and the Go `IngestIR`
  consumer validate against it; `tests/ir-conformance/check.py` is the executable
  conformance suite.
- **The flake interface**: `terraeNivis.plan = ledger → IR`, evaluated each phase
  with the outputs ledger injected.

## Testing

```sh
go test ./...                      # Go unit + e2e (uses the fake providers)
bash tests/run-nix-tests.sh        # Nix property tests + IR conformance
python3 tests/ir-conformance/check.py test   # IR schema/referential conformance
```

The milestone exit test is `tests/e2e` (`TestHeadlineRoundTrip`): two providers,
unknowns on both sides, ≥3 phases to fixpoint, a Nix consumer reading both.

## License

terrae nivis's own code is **Apache-2.0** (`LICENSE`). It is free to use commercially.
The vendored Terraform-protocol files (`proto/tfplugin{5,6}.proto` and the
generated `internal/tfplugin{5,6}` stubs) and some HashiCorp/IBM dependencies are
**MPL-2.0**, which also permits commercial use. There is **no BUSL** (the
source-available license that triggered the OpenTofu fork) anywhere in this
project. See `LICENSING.md` for the full breakdown and `NOTICE` for attributions.

---

This repository was built autonomously following a spec-driven process: see
`DESIGN.md` (architecture decisions), `ROADMAP.md` (epics), `CLAUDE.md` (the
builder's instructions), and `openspec/` (the per-change specs). The core test
suite is hermetic (fakes, no network); real-provider support (registry download +
checksum verification, tfprotov5/6) is proven against AWS — see **Real providers
(AWS)** above.
