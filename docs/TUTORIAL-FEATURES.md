# Tutorial: the daily-driver features (hands-on, no cloud)

This walks you through everything added on the road to v1, in one config you can
run **right now** against the in-repo fake providers: **no AWS account, no
credentials, no network, no cost**. You will use:

- **variables** (`mkVars`, `--var`),
- a **datasource** (read existing infra and feed it in),
- the **round trip** across phases (a value a provider computes, fed back through
  Nix into the next resource),
- **stack outputs** (`nivis output`),
- the **colored, phase-grouped plan/apply** output,
- **shell completion** and **`state pull`/`push`**.

The fake providers are deterministic, so your output will match what is shown
below exactly.

## Setup

From a checkout of the Nivis repo, enter a shell with `nivis` and the in-repo fake
providers on your `PATH`, in one command:

```sh
nix shell .#nivis .#fake-providers
```

That is all the setup. `nivis` and the fake providers (`provider-alpha`,
`provider-beta`) are now on your `PATH`, and the example config references them by
bare name, so there is nothing to build or copy.

The config for this tutorial ships with the repo as the starter
`nix/example/tutorial-features-0.4/`, exposed as the flake attribute
`nivis.tutorial`. Every command below passes `--attr nivis.tutorial`. (In your
own project you would write your own flake with `nivis.plan`; here we point at the
bundled one so there is nothing to scaffold.)

> **No repo checkout? Scaffold it into a sandbox.** `nivistutor` writes this
> tutorial's files (a ready `flake.nix`, the config, and a README) into your own
> directory, so you run it with plain `nivis` (no `--attr`). In a throwaway shell:
>
> ```sh
> nix shell github:nivis-project/nivis#nivis github:nivis-project/nivis#tutor
> nivistutor --tutorial features-0.4 --dir nivis-features
> cd nivis-features
> nivis plan --var env=prod
> ```
>
> See [the nivistutor section in Getting started](GETTING-STARTED.md#scaffold-a-tutorial-with-nivistutor).

Here is the whole config, annotated. It is one small graph that touches every
feature:

```nix
{ nivis }:
ledger:
let
  inherit (nivis) mkResource mkData str toIR mkVars;

  # VARIABLES: typed inputs. `env` is required (no default); `replicas` defaults.
  vars = mkVars {
    env = { type = "str"; };                 # required
    replicas = { type = "int"; default = 2; };
  } (ledger.vars or { });

  # DATASOURCE: read "existing" infra. The fake returns result = "found:<query>".
  lookup = mkData {
    provider = "alpha"; type = "alpha_lookup"; name = "existing";
    config = { query = vars.env; };
  };

  # A resource whose label embeds the datasource result: data flows IN.
  token = mkResource {
    provider = "alpha"; type = "alpha_token"; name = "app";
    config = { label = lookup.refAttr "result"; };
  };

  # ROUND TRIP: a beta record whose `from` is a Nix string over the token's
  # apply-time value -> resolves in a later phase.
  record = mkResource {
    provider = "beta"; type = "beta_record"; name = "app";
    config = { from = str [ "env-${vars.env}-" (token.refAttr "value") ]; };
  };
in
toIR {
  providers = {
    alpha = { source = "provider-alpha"; config = { }; };  # on $PATH (nix shell)
    beta  = { source = "provider-beta";  config = { }; };
  };
  dataSources = [ lookup ];
  resources = [ token record ];
  # OUTPUTS: named values surfaced out of the run.
  outputs = {
    env = vars.env;
    replicas = vars.replicas;
    lookupResult = lookup.refAttr "result";
    endpoint = record.refAttr "endpoint";
  };
  inherit ledger;
}
```

## 1. Variables: a required variable

`env` has no default, so it is required. Run a plan without it:

```sh
nivis plan --attr nivis.tutorial
```

```
error: nivis.mkVars: required variable 'env' is not set (declare a default or pass --var env=...)
```

That is `mkVars` refusing to proceed until you supply `env`. Set it with `--var`:

```sh
nivis plan --attr nivis.tutorial --var env=prod
```

```
  + alpha.alpha_token.app (alpha_token)
  + beta.beta_record.app (beta_record)

2 change(s) across 2 resource(s) (+ create, ~ update, -/+ replace, = no change). Run `nivis apply`.
```

On a terminal the `+` markers are green. Note the **datasource is not in the
plan**: a datasource is read, not created.

Variable precedence (lowest to highest, like Terraform): a default in Nix, then
`NIVIS_VAR_<name>`, then `--var-file`, then `--var`. So an explicit `--var`
always wins. An `int` variable accepts the string form from the CLI, so
`--var replicas=5` works.

## 2. Apply: the datasource read and the round trip, by phase

<!-- release-note: Apply shows the round trip, grouped by phase -->
A single `apply` reads a datasource, then resolves resources across phases (the
fixpoint made visible), colorised by change type:

```sh
nivis apply --attr nivis.tutorial --var env=prod
```

```
Applied 3 resource(s) across 3 phase(s):

Phase 1
  r data.alpha.alpha_lookup.existing
Phase 2
  + alpha.alpha_token.app
Phase 3
  + beta.beta_record.app
```
<!-- /release-note -->

Read the phases top to bottom, the fixpoint made visible:

- **Phase 1** reads the **datasource** (`r` = read, distinct from `+` create). Its
  result is `found:prod`.
- **Phase 2** creates the token, whose `label` is the datasource result.
- **Phase 3** creates the beta record, whose `from` is a Nix string built from the
  token's apply-time `value`. That value did not exist until phase 2, so Nix
  re-evaluated with it injected and phase 3 resolved. **That is the round trip.**

On a terminal, `r` is dim and `+` is green; piped or with `NO_COLOR` set the same
markers are plain text.

## 3. Inspect the round trip in state

```sh
nivis state show alpha.alpha_token.app
```

```
alpha.alpha_token.app (alpha_token)
  label = found:prod
  value = alpha:found:prod:0
  id = alpha-0
```

`label = found:prod` is the **datasource result that flowed into the resource**.
`value` then embeds it, and that `value` is what the beta record's `from` is built
from in the next phase.

## 4. Stack outputs

<!-- release-note: Read named outputs out of a run -->
Surface named values out of a run with `nivis output` (text, a single value, or
`--json` for a CI step):

```sh
nivis output --attr nivis.tutorial --var env=prod
```

```
endpoint = beta://env-prod-alpha:found:prod:0
env = prod
lookupResult = found:prod
replicas = 2
```
<!-- /release-note -->

- `lookupResult` is from the **datasource**,
- `endpoint` is the **round-trip** value (built across both providers and phases),
- `env` is your variable echoed out, `replicas` is the `int` default.

Print one output, or get JSON for a CI step / another stack:

```sh
nivis output endpoint --attr nivis.tutorial --var env=prod
# beta://env-prod-alpha:found:prod:0

nivis output --attr nivis.tutorial --var env=prod --json
```

```json
{
  "endpoint": "beta://env-prod-alpha:found:prod:0",
  "env": "prod",
  "lookupResult": "found:prod",
  "replicas": 2
}
```

Change the variable and watch the outputs change: `--var env=dev` gives
`lookupResult = found:dev`, and `--var replicas=5` gives `replicas = 5`.

## 5. Move state around (pull / push)

The whole state document is portable:

```sh
nivis state pull > backup.json     # whole state to a file
nivis state list                   # the resource ids in state
```

`state push` replaces state from a file or stdin; it confirms first (and requires
`--force` when piped), so you cannot clobber your state of record by accident:

```sh
nivis state push --in backup.json --force
```

If another `nivis` is running, a state command reports `state appears locked by
another nivis process (...)` and times out, instead of hanging.

## 6. Shell completion

Install tab-completion for your shell (it completes commands, flags, and resource
ids in state for `state show` / `--target`):

```sh
source <(nivis completion bash)        # bash, current shell
# zsh:  nivis completion zsh  > "${fpath[1]}/_nivis"
# fish: nivis completion fish > ~/.config/fish/completions/nivis.fish
```

## 7. Generate a provider reference

`nivis gen` turns a provider's schema into typed Nix constructors. The generated
file lists every argument (with type and required/optional), every nested block
(with the correct list-vs-single shape), and the computed outputs, so it doubles
as the per-provider argument reference:

```sh
nivis gen --provider provider-alpha --out ./generated   # provider-alpha is on $PATH
cat ./generated/alpha/alpha_token.nix
```

## 8. Tear down

```sh
nivis destroy --attr nivis.tutorial --var env=prod
```

```
Destroyed 2 resource(s):
  - beta.beta_record.app
  - alpha.alpha_token.app
```

Resources are destroyed in reverse dependency order (the record before the token).
The datasource is not destroyed, because it was only ever read.

## What you just exercised

In one config: typed **variables** with CLI overrides, a **datasource** read,
the **round trip** across three phases, **stack outputs** (text and JSON),
**phase-grouped colored** plan/apply, **state pull/push** with locking,
**completion**, and **codegen**. That is the whole "daily-driver" surface, with
no cloud account in sight. To do the same against real infrastructure, see the
[AWS S3 tutorial](TUTORIAL-AWS-S3.md) and the [EC2 + NixOS
tutorial](TUTORIAL-EC2-NIXOS.md).
