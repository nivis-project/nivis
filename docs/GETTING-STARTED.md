# Getting started with Nivis

A hands-on walkthrough using the in-repo **fake providers**. Everything here runs
**offline**: no provider registry, no cloud account, no credentials. You need
**Go 1.22+** and **Nix**.

## 1. Get the binaries on your PATH

Enter a shell with `nivis` and the in-repo fake providers, in one command:

```sh
nix shell .#nivis .#fake-providers
```

`provider-alpha` and `provider-beta` are minimal `tfprotov6` providers used as a
hermetic test substrate. Their outputs are a deterministic function of inputs (a
per-process counter seeded by `TERRAE_NIVIS_FAKE_COUNTER`, default 0), so every run is
reproducible. The example configs reference them by bare name, so once they are on
your `PATH` there is nothing to build or copy.

> Contributors who'd rather use the Go toolchain directly can build instead:
> `go build -o bin/provider-alpha ./cmd/provider-alpha` (and `provider-beta`,
> `nivis`). If you do, prepend `./bin` to your `PATH` (`export PATH=$PWD/bin:$PATH`)
> so the `nivis` and bare-name provider sources resolve just as in the Nix shell.

### Scaffold a tutorial with nivistutor

If you do **not** have a repo checkout and just want to try Nivis in a sandbox,
`nivistutor` writes a tutorial's starter files (a ready `flake.nix`, the config,
and a README) into a directory of your choosing, so you run it with plain `nivis`,
with no `--flake`/`--attr` flags. It scaffolds the files for you to read and run;
it does not run `nivis` for you (you learn by doing).

```sh
nix shell github:nivis-project/nivis#nivis github:nivis-project/nivis#tutor
nivistutor
```

It greets you, lists the available tutorials (this **getting-started** one and the
current release's **features** tutorial), asks whether to write into a new
subdirectory or the current one, writes the files, and prints the exact `nivis`
commands to run next. The `#tutor` shell carries the fake providers too, so the
scaffolded project runs immediately. Non-interactively:

```sh
nivistutor --list                                   # the available tutorials
nivistutor --tutorial getting-started --dir my-nivis # write without prompts
```

The starter's `flake.nix` is pinned to the nivis release that scaffolded it, so
the library and your `nivis` binary agree. Existing files are never overwritten
without `--force`.

## 2. The example configuration

The flake's `nivis.plan` (in `nix/example/`) describes three resources and a
consumer, wired so each hop crosses the Nix boundary:

```
alpha_token.A            (alpha)            -- no inputs; A.value computed at apply
   └─ name = "rec-" + A.value               (a __derived value)
beta_record.B  (beta)    from = name        -- B.endpoint computed at apply
   └─ final = B.endpoint + "::" + A.value    (a __derived on BOTH providers)
alpha_token.C  (alpha)   label = final

systemConfig (a Nix consumer) reads:
  recordEndpoint = B.endpoint   # from beta
  tokenValue     = A.value      # from alpha
  combined       = final        # from both
```

Because `name` and `final` are values Nix *computes from* provider outputs, they
can't be known until those outputs exist and Nix is re-evaluated, which is what
forces multiple phases.

## 3. Plan and apply

```sh
nivis plan
```

```
  + alpha.alpha_token.A (alpha_token)
  + beta.beta_record.B (beta_record)
  + alpha.alpha_token.C (alpha_token)

3 change(s) across 3 resource(s) (+ create, ~ update, -/+ replace, = no change). Run `nivis apply`.
```

```sh
nivis apply
```

```
Applied 3 resource(s) across 3 phase(s):

Phase 1
  + alpha.alpha_token.A
Phase 2
  + beta.beta_record.B
Phase 3
  + alpha.alpha_token.C
```

Three phases, not one: phase 1 applies A (nothing else is ready); re-evaluating
with `A.value` known unlocks B; re-evaluating with `B.endpoint` known unlocks C.
The loop halts at a fixpoint once nothing new resolves.

## 4. Inspect the round trip

```sh
nivis state list
nivis state show alpha.alpha_token.C
```

```
alpha.alpha_token.C (alpha_token)
  id = alpha-1
  label = beta://rec-alpha::0::alpha::0
  value = alpha:beta://rec-alpha::0::alpha::0:1
```

`C.label` is `final`: a string Nix built from **both** `B.endpoint` (beta) and
`A.value` (alpha). That value only became concrete after both providers applied
and Nix re-evaluated. That is the round trip.

## Stack outputs

To surface named values out of a run (the Terraform `output "x" {}` equivalent),
declare them with the `outputs` argument to `toIR`:

```nix
toIR {
  providers = { ... };
  resources = [ A B C ];
  outputs = {
    token = A.refAttr "value";          # from one resource
    combined = final;                   # composed across both providers
  };
  inherit ledger;
}
```

Read them after apply with `nivis output` (resolved from current state):

```sh
nivis output
```

```
combined = beta://rec-alpha::0::alpha::0
token = alpha::0
```

`nivis output <name>` prints a single value, and `nivis output --json` prints a
JSON object (`{ "<name>": <value> }`) for a CI step or another stack to consume.
Outputs reuse the round trip's resolution, so a value composed across providers
and phases comes back concrete.

## 5. Refresh and destroy

```sh
nivis refresh    # reconciles state via ReadResource; no changes here
nivis destroy    # tears down in reverse dependency order
```

```
Destroyed 3 resource(s):
  - alpha.alpha_token.C
  - beta.beta_record.B
  - alpha.alpha_token.A
```

## 6. Generate constructors from a provider schema

`nivis gen` turns any provider's schema into typed Nix constructors:

```sh
nivis gen --provider provider-alpha --out ./generated
cat ./generated/alpha/alpha_token.nix
```

The generated constructor requires the provider's required inputs (throwing a
named error if missing), passes optional inputs through, omits computed-only
attributes (they're outputs), and accepts an `overrides` argument so you can
adjust the generated output.

It also includes the resource's **nested blocks**, each with the shape that
matches its nesting, so you never guess list-vs-single: a list/set-nested block
(e.g. `ingress`, `disk_container`) is an argument defaulting to `[]` and written
`[ { ... } ]`; a single-nested block is a plain attrset; a map-nested block is a
map of attrsets. Every block is documented in the generated file with its nesting
and inner attribute names.

So **the generated constructor is your per-provider argument reference**: instead
of translating a provider's Terraform docs into Nivis terms by hand, run `nivis
gen` and read the `.nix`. It lists every argument (with type and
required/optional), every nested block (with its nesting), and the computed
outputs, all in Nivis form.

## 7. A real provider (AWS)

<!-- ANCHOR: aws -->
Everything above is offline against the fakes. The same `nivis` commands drive
**real** providers: `nivis` resolves a provider by address from the OpenTofu
registry, downloads and checksum-verifies the binary, negotiates the plugin
protocol (AWS speaks v5), configures it, and runs plan/apply/destroy. The example
`nix/example/aws.nix` (flake attr `nivis.aws`) declares the `hashicorp/aws`
provider with `mkProvider` and one `aws_s3_bucket`.

> ⚠️ **This creates a real resource in your AWS account**: one (free-tier) S3
> bucket, then destroys it. The provider's `region` lives in the Nix config; only
> credentials come from the environment (the AWS SDK default chain), so set
> `AWS_PROFILE` (or `AWS_ACCESS_KEY_ID`/…). The first run downloads the
> ~900&nbsp;MB AWS provider (cached afterwards).

For the full, hand-held walkthrough (prerequisites, writing the config line by
line, `plan`/`apply`/inspecting state/`destroy`, and troubleshooting) follow the
**[AWS S3 tutorial](TUTORIAL-AWS-S3.md)**.
<!-- ANCHOR_END: aws -->

## Where to go next

- `IR-CONTRACT.md` + `ir-schema.json`: the IR, the stable contract
  between the Nix frontend and the Go executor.
- `TESTING.md`: the test layers and the headline two-provider e2e.
- `DESIGN.md`: why the architecture is the way it is (spawn-not-link,
  batch-not-live, phased re-eval to a fixpoint).

The core test suite is hermetic (fakes, no network/credentials); real-provider
support (registry download + checksum verification, tfprotov5/6) is proven
against AWS as shown above.
