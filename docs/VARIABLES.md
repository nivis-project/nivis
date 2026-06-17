# Variables

Variables let you parameterise a Nivis configuration instead of hard-coding it:
declare them in Nix with `nivis.mkVars`, read them in your plan, and set them per
run from the command line, a file, or the environment. This is how you keep one
configuration and vary the region, a name suffix, an instance size, or any other
input between environments.

## Declare variables with `mkVars`

`mkVars` takes a declaration attrset (each variable with an optional `type` and
`default`) and the values Nivis injects (`ledger.vars`), and returns the resolved,
validated values your plan reads:

```nix
nivis.plan =
  ledger:
  let
    vars = lib.mkVars {
      region = { type = "str"; default = "eu-central-1"; };
      suffix = { type = "str"; };          # no default -> required
      replicas = { type = "int"; default = 2; };
    } (ledger.vars or { });
  in
  lib.toIR {
    providers.aws = lib.mkProvider {
      source = "registry.opentofu.org/hashicorp/aws";
      config.region = vars.region;
    };
    resources = [
      (lib.mkResource {
        provider = "aws"; type = "aws_s3_bucket"; name = "demo";
        config = { bucket = "myapp-${vars.suffix}"; force_destroy = true; };
      })
    ];
    inherit ledger;
  };
```

Read each variable as `vars.<name>`. Pass `(ledger.vars or { })` so a run with no
variables set still evaluates (the declared defaults fill in).

### Types

`type` is one of:

| Type | Accepts |
| --- | --- |
| `str` | a string |
| `int` | an integer |
| `bool` | a boolean |
| `any` | anything (no validation); the default if `type` is omitted |

A value whose type does not match its declaration is an error that names the
variable and the expected type.

### Defaults and required variables

- A variable **with a `default`** uses that default when unset.
- A variable **without a `default`** is **required**: if it is unset when the
  config reads it, evaluation fails with an error naming the variable. This is how
  you make a configuration refuse to run until a needed value is supplied.

## Set variables (and precedence)

A variable's value is resolved from these sources, listed **lowest to highest
priority** (a higher source overrides a lower one for the same name):

1. the **default** declared in Nix;
2. the environment variable **`NIVIS_VAR_<name>`**;
3. a **`--var-file <file>`** (a JSON object; when given more than once, a later
   file overrides an earlier one);
4. a **`--var name=value`** flag (when given more than once, a later flag
   overrides an earlier one).

So an explicit `--var` on the command line always wins. This is Terraform's
precedence: it avoids a stale environment variable silently overriding what you
typed, and it matches the mental model if you are coming from Terraform or
OpenTofu.

```sh
# default in Nix (eu-central-1), overridden per run:
nivis plan --var region=us-east-1

# from a file (later --var-file wins; an explicit --var still beats the file):
nivis apply --var-file prod.json --var suffix=prod

# from the environment (lowest override; a file or flag beats it):
NIVIS_VAR_region=eu-west-1 nivis plan
```

A `--var-file` is a JSON object, so it can carry non-string values for `int` /
`bool` / `any` variables:

```json
{ "region": "us-east-1", "replicas": 4, "enabled": true }
```

### Errors

- A `--var` without `=`, or with an empty name, is rejected with a message naming
  the offending flag.
- A `--var-file` that is missing or is not a JSON object is rejected naming the
  file.
- A required variable that is never set is an evaluation error naming it.

## Purity and secrets

Variable values travel only inside the executor's `0600` ledger file (the same
file that carries resolved outputs), never on the Nix command line and never into
the Nix store. The Nix evaluation reads them as plain data with
`builtins.fromJSON`; it does not read your environment. So variables introduce no
impurity and no new path for a secret to leak into a world-readable store.

> Variables are **not** yet a secrets mechanism: there is no "sensitive variable"
> marking in this version. Do not treat a plain variable as a secure secret store;
> a dedicated secrets integration is planned separately.

## How variables fit the phased-eval loop

Variables are **known inputs**, resolved once before the first phase and injected
unchanged on every phase (unlike resource outputs, which accumulate across phases
to a fixpoint). A variable value is therefore always concrete: it is never a
cross-resource reference or an unknown placeholder. In the injected ledger they
appear as a `vars` object alongside `outputs` (see `docs/IR-CONTRACT.md`).

## What is not here yet

- **Module-system declaration** (NixOS-style `options.vars` via `lib.mkOption`).
  `mkVars` is the current API; resolved values land in `ledger.vars`, the same
  place a future module-options layer would write to, so it can be added later
  without changing how you set variables.
- **Rich types** beyond `str` / `int` / `bool` / `any` (lists, attrsets, enums).
- **Automatic file loading** by name (a `.auto`-style convention); use an explicit
  `--var-file`.
- **Sensitive variables**; see the note above.
