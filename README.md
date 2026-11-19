<p align="center">
  <img src="https://nivis.tf/social.png" alt="Nivis: Terraform providers as first-class Nix values." width="100%">
</p>

<p align="center">
  <a href="https://github.com/nivis-project/nivis/releases/latest"><img src="https://img.shields.io/github/v/release/nivis-project/nivis?color=2d8dd2" alt="Latest release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache--2.0-2d8dd2" alt="License: Apache-2.0"></a>
  <a href="https://github.com/nivis-project/nivis/actions/workflows/ci.yml"><img src="https://github.com/nivis-project/nivis/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://nixos.org"><img src="https://img.shields.io/badge/built%20with-Nix-2d8dd2?logo=nixos&logoColor=white" alt="Built with Nix"></a>
</p>

# Nivis

**Terraform providers as first-class Nix values.**

Describe real infrastructure as Nix instead of HCL. A thin Go executor drives
unmodified Terraform and OpenTofu provider binaries, and what the cloud returns
flows back into Nix, which re-evaluates until nothing is left unknown.

<p align="center">
  <img src="assets/demo.gif" alt="nivis plan, then nivis apply across three phases, then nivis output showing a value Nix combined from what two providers returned" width="720">
</p>

**Status: early but real.** The round trip works across providers, and AWS
applies, updates, replaces and destroys today. Expect rough edges; the IR and the
flake interface are the stable parts.

[Website](https://nivis.tf) · [Quick start](#quickstart) · [Docs](https://docs.nivis.tf/) · [Registry](https://registry.nivis.tf) · [Discussions](https://github.com/nivis-project/nivis/discussions)

*Nivis is Latin for "of snow". Formerly `nixform`, then Terrae Nivis.*

## You already know half of it

**Coming from Nix?** The missing half is the provider ecosystem. A provider is a
plugin binary that speaks to one cloud API, and Nivis runs any
OpenTofu-compatible provider as-is. A resource is an attrset built with
`mkResource`; compose it with the functions and modules you already write.

**Coming from Terraform?** The missing half is the language. Same providers, same
plan, apply and destroy, but a `flake.nix` takes the place of your `.tf` files,
and `aws_s3_bucket.demo.id` becomes `bucket.refAttr "id"`: an ordinary value you
can pass to any function. Nix also builds machines, so a NixOS image and the
cloud resources that boot it can live in one expression.

<table>
<tr><th>main.tf</th><th>flake.nix (excerpt)</th></tr>
<tr>
<td>

```hcl
provider "aws" {
  region = "eu-central-1"
}

resource "aws_s3_bucket" "demo" {
  force_destroy = true
}
```

</td>
<td>

```nix
providers.aws = lib.mkProvider {
  source = "registry.opentofu.org/hashicorp/aws";
  config.region = "eu-central-1";
};
resources = [
  (lib.mkResource {
    provider = "aws"; type = "aws_s3_bucket"; name = "demo";
    config.force_destroy = true;
  })
];
```

</td>
</tr>
</table>

What Nix adds is the **round trip**: a value the cloud generates (an IP, an ID, a
name) comes back into Nix, which re-evaluates to build the configuration that
depends on it, phase by phase, until nothing is left unknown. The
[AWS tutorial](docs/TUTORIAL-AWS-S3.md) shows it end to end.

## Quickstart

You need **Nix** (with flakes). The first two steps touch no cloud account.

Run Nivis straight from the flake, no checkout:

```sh
nix run github:nivis-project/nivis#nivis -- --version
```

Take the guided tour. `nivistutor` scaffolds a tutorial project that runs
against in-repo fake providers, so you can watch a plan and an apply before any
credentials are involved:

```sh
nix shell github:nivis-project/nivis#nivis github:nivis-project/nivis#tutor
nivistutor
```

Then point Nivis at a flake that describes your infra. A minimal one:

```nix
{
  inputs.nivis.url = "github:nivis-project/nivis";

  outputs = { self, nivis }:
    let lib = nivis.lib; in {
      # `nivis` evaluates this attribute by default.
      nivis.plan = ledger: lib.toIR {
        providers.aws = lib.mkProvider {
          source = "registry.opentofu.org/hashicorp/aws";
          config.region = "eu-central-1";
        };
        resources = [
          (lib.mkResource {
            provider = "aws"; type = "aws_s3_bucket"; name = "demo";
            config.force_destroy = true;
          })
        ];
        inherit ledger;
      };
    };
}
```

From that directory, creating real resources in your own AWS account:

```sh
nivis plan      # show what will change   (+ create  ~ update  -/+ replace  = no change)
nivis apply     # apply, resolving across phases to a fixpoint
nivis state show <resource-id>
nivis destroy
```

(`nivis` shells out to `nix` to evaluate your config, and fetches and
checksum-verifies real providers from the OpenTofu registry on first use.)

- **New here?** The **[getting-started guide](docs/GETTING-STARTED.md)** walks the
  offline demo (in-repo fake providers, no cloud) and schema codegen (`nivis gen`).
- **Real infra?** The **[AWS S3 tutorial](docs/TUTORIAL-AWS-S3.md)** builds a
  bucket and a file whose contents Nix generates from the bucket's name (the
  round trip), from an empty directory.
- **Installing `nivis`** (`nix shell`, `nix profile`): see
  **[docs/INSTALL.md](docs/INSTALL.md)**.
- Browse it all on the **[docs site](https://docs.nivis.tf/)**.

## How it works

Nix evaluates your configuration to a JSON **IR**. Values that aren't known until
apply-time are emitted as typed placeholders, a `__ref` (another resource's
output) or a `__derived` (a value Nix *computed* from an output). The executor
ingests the IR, drives the providers, gathers their apply-time outputs into a
**ledger**, then **re-evaluates Nix** with that ledger injected so placeholders
resolve; deeper chains take more phases, to a fixpoint. The full reasoning and
why this (not an `Output<T>` promise model) is the honest, Nix-shaped approach
is in **[docs/OVERVIEW.md](docs/OVERVIEW.md)** and
**[docs/DESIGN.md](docs/DESIGN.md)**.

## The Nix library

`nivis.lib` is pure (builtins only, no nixpkgs, so it evaluates without the
binary cache):

- `mkResource { provider; type; name; config; }`: a resource, with output
  references (`refAttr`) for wiring one resource into another.
- `mkData { provider; type; name; config; }`: a datasource (existing infra to
  read, not create); its outputs feed resources via `refAttr`, read per phase.
  See **[docs/DATASOURCES.md](docs/DATASOURCES.md)**.
- `mkProvider { source; config; }`: a provider, config (incl. nested blocks) in
  Nix; flows into the provider's `Configure`.
- `str`/`derived`: build values from provider outputs (the round trip).
- `mkVars`: declare typed config variables with defaults (required when no
  default), resolved from `--var` / `--var-file` / `NIVIS_VAR_*` (an explicit
  `--var` wins). Read them as `vars.<name>` in the plan.
- `drv`/`drvFile`: mark a Nix build output (a derivation) so `nivis apply`
  realises it before use; `drv d` uses `d.passthru.filePath` when present,
  `drvFile d "sub/path"` names a file explicitly. (See the EC2 tutorial.)
- `toIR`: serialize to the IR; `mkResources` (`count`/`for_each`) and
  `evalModules` (module composition).

`nivis gen --provider <binary> --out <dir>` generates typed constructors from any
provider's schema, including nested blocks (with the right list-vs-single shape).
The generated constructor doubles as the per-provider argument reference.

## Stable contracts

Versioned interfaces, change the spec before the shape:

- **The IR**: `docs/IR-CONTRACT.md` + `docs/ir-schema.json` (the normative JSON
  Schema). Both the Nix `toIR` producer and the Go consumer validate against it.
- **The flake interface**: `nivis.plan = ledger → IR`, evaluated each phase with
  the outputs ledger injected.

## How it compares

> **The round trip is not unique to Nivis, and we say so.** NixOps 4 feeds
> provider outputs back into Nix too, structural dependencies included. What is
> particular here is that the Terraform-provider coupling *is* the whole product:
> unmodified provider binaries, fetched and checksum-verified from the OpenTofu
> registry, so every provider works on day one.

The **[comparison](docs/COMPARISON.md)** sets Nivis next to Terranix,
OpenTofu/Terraform, Pulumi, NixOps, CDK and CloudFormation, including where
Nivis is still young.

## Questions & feedback

Ask questions, share what you built or propose an idea in
**[Discussions](https://github.com/nivis-project/nivis/discussions)**. Report a
bug or a provider that misbehaves in
**[Issues](https://github.com/nivis-project/nivis/issues)**.

## Contributing

Nivis is built with the standard Go toolchain; the flake (`nix build .#nivis`,
`nix run .#nivis`) is the packaged path. From a checkout:

```sh
go build -o bin/nivis ./cmd/nivis     # the CLI
go test ./...                         # Go unit + e2e (uses the in-repo fakes)
bash tests/run-nix-tests.sh           # Nix property tests + IR conformance
bash tests/check-docs-ssot.sh         # docs single-source-of-truth
```

The repo follows a spec-driven process guided by `openspec/` (the per-change
specs). Releases: `docs/RELEASING.md`.

## License

Nivis's own code is **Apache-2.0** (`LICENSE`). See `LICENSING.md` for the
breakdown and `NOTICE` for attributions.
