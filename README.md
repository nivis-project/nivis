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

## Quickstart (offline, no credentials)

Everything here runs against in-repo **fake providers** — no registry, no cloud
account. You need Go 1.22+ and Nix.

```sh
go build -o bin/provider-alpha ./cmd/provider-alpha
go build -o bin/provider-beta  ./cmd/provider-beta
go build -o bin/tn ./cmd/tn

./bin/tn plan      # plan the headline topology
./bin/tn apply     # resolves across 3 phases to a fixpoint
./bin/tn state show alpha.alpha_token.C   # a value Nix derived from BOTH providers
./bin/tn destroy
```

Prefer Nix? `nix run .#tn -- plan` / `apply` (and `nix build .#tn`) build the CLI
from source — the library outputs stay pure-builtins.

That's the round trip in miniature. For the full story:

- **[`docs/OVERVIEW.md`](docs/OVERVIEW.md)** — how it works (the IR, the outputs
  ledger, phased re-evaluation to a fixpoint).
- **[`docs/GETTING-STARTED.md`](docs/GETTING-STARTED.md)** — the guided
  walkthrough: the example topology, plan/apply/inspect/destroy, schema codegen
  with `tn-gen`, **and driving a real provider (AWS)**.
- **[Docs site](https://wearetechnative.github.io/terrae-nivis/)** — the same
  docs, browsable.

> Terrae Nivis also drives **real** providers (registry fetch, checksum
> verification, plugin protocol v5/v6, plan/apply/destroy) — see the AWS
> walkthrough in `docs/GETTING-STARTED.md`. ⚠️ that creates a real cloud resource.

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

## Docs site

A branded [mdBook](https://rust-lang.github.io/mdBook/) site lives in
`docs-site/`; it reuses these Markdown docs (one source of truth) and applies the
brand theme (`docs/BRAND.md`). It is published to GitHub Pages at
**<https://wearetechnative.github.io/terrae-nivis/>** (deployed by
`.github/workflows/docs.yml` on every push to `main`). Build it locally with:

```sh
cargo install mdbook        # if you don't have it (single static binary)
mdbook build docs-site      # -> docs-site/book/   (mdbook serve for live preview)
```

See `docs-site/README.md` for details.

## Testing

```sh
go test ./...                      # Go unit + e2e (uses the fake providers)
bash tests/run-nix-tests.sh        # Nix property tests + IR conformance
python3 tests/ir-conformance/check.py test   # IR schema/referential conformance
bash tests/check-docs-ssot.sh      # docs single-source-of-truth (no duplicated sections)
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
checksum verification, tfprotov5/6) is proven against AWS — see the AWS
walkthrough in `docs/GETTING-STARTED.md`.
