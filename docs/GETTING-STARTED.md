# Getting started with terrae nivis

A hands-on walkthrough using the in-repo **fake providers**. Everything here runs
**offline** — no provider registry, no cloud account, no credentials. You need
**Go 1.22+** and **Nix**.

## 1. Build the binaries

```sh
go build -o bin/provider-alpha ./cmd/provider-alpha
go build -o bin/provider-beta  ./cmd/provider-beta
go build -o bin/tn ./cmd/tn
```

`provider-alpha` and `provider-beta` are minimal `tfprotov6` providers used as a
hermetic test substrate. Their outputs are a deterministic function of inputs (a
per-process counter seeded by `TERRAE_NIVIS_FAKE_COUNTER`, default 0), so every run is
reproducible.

> Prefer Nix? The flake builds the CLIs from source: `nix run .#tn -- …` and
> `nix run .#tn-gen -- …` (or `nix build .#tn`). Everywhere below, `./bin/tn` can
> be read as `nix run .#tn --`. You still build the fake providers with `go build`
> (they aren't packaged as apps).

## 2. The example configuration

The flake's `terraeNivis.plan` (in `nix/example/`) describes three resources and a
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
can't be known until those outputs exist and Nix is re-evaluated — which is what
forces multiple phases.

## 3. Plan and apply

```sh
./bin/tn plan
```

```
+ alpha.alpha_token.A (alpha_token)
+ beta.beta_record.B (beta_record)
+ alpha.alpha_token.C (alpha_token)

3 resource(s) to resolve across phases. Run `tn apply`.
```

```sh
./bin/tn apply
```

```
Applied 3 resource(s) across 3 phase(s):
  ✓ alpha.alpha_token.A
  ✓ beta.beta_record.B
  ✓ alpha.alpha_token.C
```

Three phases, not one: phase 1 applies A (nothing else is ready); re-evaluating
with `A.value` known unlocks B; re-evaluating with `B.endpoint` known unlocks C.
The loop halts at a fixpoint once nothing new resolves.

## 4. Inspect the round trip

```sh
./bin/tn state list
./bin/tn state show alpha.alpha_token.C
```

```
alpha.alpha_token.C (alpha_token)
  id = alpha-1
  label = beta://rec-alpha::0::alpha::0
  value = alpha:beta://rec-alpha::0::alpha::0:1
```

`C.label` is `final` — a string Nix built from **both** `B.endpoint` (beta) and
`A.value` (alpha). That value only became concrete after both providers applied
and Nix re-evaluated. That is the round trip.

## 5. Refresh and destroy

```sh
./bin/tn refresh    # reconciles state via ReadResource; no changes here
./bin/tn destroy    # tears down in reverse dependency order
```

```
Destroyed 3 resource(s):
  - alpha.alpha_token.C
  - beta.beta_record.B
  - alpha.alpha_token.A
```

## 6. Generate constructors from a provider schema

`tn-gen` turns any provider's schema into typed Nix constructors:

```sh
go run ./cmd/tn-gen -- --provider ./bin/provider-alpha --out ./generated
cat ./generated/alpha/alpha_token.nix
```

The generated constructor requires the provider's required inputs (throwing a
named error if missing), passes optional inputs through, omits computed-only
attributes (they're outputs), and accepts an `overrides` argument so you can
adjust the generated output.

## 7. A real provider (AWS)

<!-- ANCHOR: aws -->
Everything above is offline against the fakes. The same `tn` commands drive
**real** providers — `tn` resolves a provider by address from the OpenTofu
registry, downloads and checksum-verifies the binary, negotiates the plugin
protocol (AWS speaks v5), configures it, and runs plan/apply/destroy. The example
`nix/example/aws.nix` (flake attr `terraeNivis.aws`) declares the `hashicorp/aws`
provider with `mkProvider` and one `aws_s3_bucket`.

> ⚠️ **This creates a real resource in your AWS account** — one (free-tier) S3
> bucket, then destroys it. The provider's `region` lives in the Nix config; only
> credentials come from the environment (the AWS SDK default chain), so set
> `AWS_PROFILE` (or `AWS_ACCESS_KEY_ID`/…). The first run downloads the
> ~900&nbsp;MB AWS provider (cached afterwards).

For the full, hand-held walkthrough — prerequisites, writing the config line by
line, `plan`/`apply`/inspecting state/`destroy`, and troubleshooting — follow the
**[AWS S3 tutorial](TUTORIAL-AWS-S3.md)**.
<!-- ANCHOR_END: aws -->

## Where to go next

- `IR-CONTRACT.md` + `ir-schema.json` — the IR, the stable contract
  between the Nix frontend and the Go executor.
- `TESTING.md` — the test layers and the headline two-provider e2e.
- `DESIGN.md` — why the architecture is the way it is (spawn-not-link,
  batch-not-live, phased re-eval to a fixpoint).

The core test suite is hermetic (fakes, no network/credentials); real-provider
support (registry download + checksum verification, tfprotov5/6) is proven
against AWS as shown above.
