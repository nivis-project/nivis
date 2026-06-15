# Real providers (AWS)

Terrae Nivis drives **real** Terraform/OpenTofu providers, not just the in-repo
fakes. It resolves a provider by address from the OpenTofu registry, downloads
and **checksum-verifies** the binary from its release host, negotiates the plugin
protocol (v5 or v6), configures it, and runs the same plan/apply/destroy cycle
you saw in [Getting started](./getting-started.md).

> ⚠️ **This creates a real resource in your AWS account.** The example creates a
> single (free-tier) S3 bucket and then destroys it. Provider settings like
> `region` live in the **Nix config** (via `mkProvider`); only credentials come
> from the environment (the AWS SDK default chain) — set `AWS_PROFILE` (or
> `AWS_ACCESS_KEY_ID`/…). First run downloads the ~900&nbsp;MB AWS provider
> (cached after).

```sh
export AWS_PROFILE=your-profile          # credentials only; region is in the Nix config

tn plan    --attr terraeNivis.aws        # show the planned bucket
tn apply   --attr terraeNivis.aws        # create a real S3 bucket (AWS-generated name)
tn state show aws.aws_s3_bucket.demo
tn destroy --attr terraeNivis.aws        # delete it
```

## Provider config lives in Nix

The `terraeNivis.aws` flake attribute (`nix/example/aws.nix`) declares the
provider with `mkProvider`, so non-secret settings such as `region` and
`default_tags` are expressed in Nix and flow into the provider's `Configure`
call:

```nix
providers.aws = mkProvider {
  source = "registry.opentofu.org/hashicorp/aws";
  config = {
    region = "eu-central-1";             # provider config in Nix, not env
    default_tags = { tags = { managed-by = "terrae-nivis"; }; };
  };
};
```

`mkProvider`'s `config` is a raw attribute tree the provider validates at
`Configure` time; nested blocks (`default_tags`, `assume_role`, `endpoints`, …)
are just nested attrsets/lists, and `toIR` resolves any `__ref`/`__derived`
leaves in it against the outputs ledger exactly as it does for resource config.
Point `region`, `source`, or the resource at anything else to drive a different
provider/setting/resource the same way.

Secrets (access keys) are intentionally **not** expressed in Nix — they come
from the environment / the AWS SDK default credential chain.
