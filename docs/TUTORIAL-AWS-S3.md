# Tutorial: an S3 bucket on AWS

A genuinely from-scratch walkthrough. You start in an **empty directory on your
own machine** — not a checkout of terrae nivis — install the `tn` CLI, scaffold a
fresh flake that *uses* terrae nivis as a dependency, declare one S3 bucket, and
drive it through `plan → apply → inspect → destroy`. By the end you'll have a
small infra flake you own and a real bucket created and torn down.

> ⚠️ **This creates a real resource in your AWS account** — a single S3 bucket
> (no objects, negligible cost) that you destroy at the end. The commands and
> outputs below come from real runs.

**Prerequisites:** Nix (with flakes enabled) on your `PATH`, and AWS credentials
you can use locally.

## Part 1 — Install `tn`

The CLI is `tn`. You don't need to clone anything; the quickest path is to run it
straight from the flake:

```sh
nix run github:wearetechnative/terrae-nivis#tn -- --version
```

If you'd rather have `tn` on your `PATH` for the rest of this tutorial, install it
persistently or open a shell with it — see **[Installing terrae
nivis](INSTALL.md)** for all the options (`nix run`, `nix shell`, `nix profile
install`, building from a clone). The rest of this tutorial writes `tn …`; if you
chose the ad-hoc form, read that as `nix run github:wearetechnative/terrae-nivis#tn -- …`.

## Part 2 — A fresh infra flake

### 2.1 Scaffold the flake

```sh
mkdir my-infra && cd my-infra
nix flake init
```

`nix flake init` drops a placeholder `flake.nix` (a `hello` package). Replace its
contents with the infra flake below.

### 2.2 The boilerplate

```nix
{
  description = "My infrastructure, as Nix code (terrae nivis).";

  # Pull terrae nivis in as a dependency. `nix flake lock` (run automatically by
  # the first tn command) records the exact revision in flake.lock.
  inputs.terrae-nivis.url = "github:wearetechnative/terrae-nivis";

  outputs =
    { self, terrae-nivis }:
    let
      # tn is the terrae nivis Nix library: mkResource, mkProvider, toIR, …
      tn = terrae-nivis.lib;
    in
    {
      # `tn` looks for the attribute `terraeNivis.plan` by default. It's a
      # function of the outputs ledger (the apply-time values fed back in each
      # phase); for a single bucket there's just one phase, so we ignore it.
      terraeNivis.plan =
        ledger:
        tn.toIR {
          # --- providers --------------------------------------------------
          providers.aws = tn.mkProvider {
            source = "registry.opentofu.org/hashicorp/aws";
            config = {
              region = "eu-central-1"; # set in Nix, not via AWS_REGION
              # default_tags is a *list-nested* block in the AWS provider, so it
              # takes a list (a bare attrset is rejected at configure time).
              default_tags = [ { tags = { managed-by = "terrae-nivis"; }; } ];
            };
          };

          # --- resources --------------------------------------------------
          resources = [
            (tn.mkResource {
              provider = "aws";
              type = "aws_s3_bucket";
              name = "demo"; # id becomes aws.aws_s3_bucket.demo
              config = {
                force_destroy = true; # let `tn destroy` delete it even if non-empty
                # `bucket` is omitted, so AWS generates a globally-unique name.
              };
            })
          ];

          inherit ledger;
        };
    };
}
```

Reading it:

- **`inputs.terrae-nivis.url`** makes terrae nivis a dependency; `tn = terrae-nivis.lib`
  binds its Nix library.
- **`terraeNivis.plan`** is the attribute `tn` evaluates by default — a function
  `ledger → IR`. (Name it something else and pass `tn plan --attr <name>`.)
- **`mkProvider`** declares the AWS provider: `source` is its registry address,
  `region` lives in Nix, and `default_tags` is a one-element list because that
  block is list-nested in the AWS provider.
- **`mkResource`** declares one `aws_s3_bucket` with a stable id
  `aws.aws_s3_bucket.demo`; `force_destroy` makes teardown easy and omitting
  `bucket` lets AWS pick a unique name.

### 2.3 Credentials

`tn` uses the **AWS SDK default credential chain** (the same one the AWS CLI
uses). Point it at your account — typically a named profile:

```sh
export AWS_PROFILE=your-profile
aws sts get-caller-identity   # sanity check
```

Only credentials come from the environment; the **region** is in the flake above.

## Part 3 — Plan, apply, inspect, destroy

Run these from your `my-infra` directory.

### Plan

```sh
tn plan
```

```
+ aws.aws_s3_bucket.demo (aws_s3_bucket)

1 resource(s) to resolve across phases. Run `tn apply`.
```

The first `tn` command resolves the `terrae-nivis` input (writing `flake.lock`)
and, on first use of a real provider, downloads it — so the first run is slower.

### Apply

```sh
tn apply
```

```
Applied 1 resource(s) across 1 phase(s):
  ✓ aws.aws_s3_bucket.demo
```

The bucket now exists. `tn` writes the resulting state to
`terrae-nivis.state.json` in `my-infra`.

### Inspect the round trip

```sh
tn state show aws.aws_s3_bucket.demo
```

```
  arn = arn:aws:s3:::terraform-20260615165907082000000001
  bucket_regional_domain_name = terraform-20260615165907082000000001.s3.eu-central-1.amazonaws.com
  tags_all = map[managed-by:terrae-nivis]
  force_destroy = true
  region = eu-central-1
  id = terraform-20260615165907082000000001
  …
```

The bucket name (`terraform-2026…`) was **generated by AWS** and read back into
state — a value that didn't exist until apply is now concrete. `tags_all` shows
the provider's `default_tags` were applied.

### Destroy

```sh
tn destroy
```

```
Destroyed 1 resource(s):
  - aws.aws_s3_bucket.demo
```

Confirm nothing's left:

```sh
aws s3api list-buckets --query 'Buckets[?contains(Name, `terraform-`)].Name'
```

## Make it your own

- **A specific bucket name:** add `bucket = "globally-unique-name";` to the
  resource `config`.
- **A different region:** change `region` in the provider `config`.
- **More resources:** add more `mkResource` entries to the `resources` list; wire
  one resource's output into another with the reference helpers (`refAttr`) and
  terrae nivis resolves them across phases.
- **Pin terrae nivis:** the input floats on the default branch; `flake.lock` pins
  the exact revision. Re-pin deliberately with `nix flake update terrae-nivis`.

## Troubleshooting

- **`NoCredentialProviders` / `could not find credentials`** — the SDK chain found
  nothing. Set `AWS_PROFILE` (or the access-key vars) and confirm with `aws sts
  get-caller-identity`.
- **`expected array … got map[string]interface {}` for a provider block** — that
  block is *list-nested*; wrap it in a list, e.g.
  `default_tags = [ { tags = { … }; } ]`.
- **First run is slow / seems to hang** — it's resolving the flake input and
  downloading the ~900&nbsp;MB AWS provider once; later runs use the cache.
- **`BucketAlreadyExists`** — you set an explicit `bucket` name someone already
  owns (S3 names are global). Omit `bucket`, or pick another.
- **`tn` can't find your flake** — run `tn` from the directory containing
  `flake.nix`, or pass `tn plan --flake /path/to/my-infra`.
