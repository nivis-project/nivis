# Importing existing infrastructure

`nivis import` adopts infrastructure that already exists into nivis state, so a
stack can be managed without being recreated. It creates nothing: the only thing
it writes is state.

It exists for migrations. Moving a running estate into nivis means its resources
must appear in state exactly as they are, because the alternative is destroying
and recreating live infrastructure.

## What it does not do

**It does not write Nix configuration.** Every resource you import must already
be declared, because nivis only manages what the configuration declares: a state
entry nivis does not know about is never planned, never applied and never
destroyed. Importing one would leave you believing something is managed when
nothing will ever touch it.

So a migration writes Nix for its resources either way. Import fills in state for
what you have declared.

## The manifest

Import takes a JSON file mapping each nivis resource id to the provider's own
identifier for the object:

```json
{
  "imports": {
    "aws.aws_route53_zone.main": { "id": "Z0042665NOVUJHACYP2Q" },
    "aws.aws_s3_bucket.assets":  { "id": "my-assets-bucket" },
    "aws.aws_iam_user.deploy":   { "id": "deploy" }
  }
}
```

```sh
nivis import --manifest imports.json
```

A manifest rather than a flag per resource, because a migration is hundreds of
resources and a command per resource is unworkable. The file is also what makes
a run resumable: fix the entries that were refused and run it again, and the ones
that succeeded are refused as already in state.

The identifier is whatever the provider expects, and nivis does not interpret it.
A Route53 zone is addressed by its zone id, an S3 bucket by its name, an IAM user
by its name. The provider decides.

## Attributes a provider will not give back

Some attributes exist only at creation. `aws_iam_access_key` returns `secret` and
`ses_smtp_password_v4` once, when the key is made, and never again. A read after
import cannot recover them, and nothing in the provider protocol marks them as
such, so the manifest has to say which they are:

```json
{
  "imports": {
    "aws.aws_iam_access_key.deploy": {
      "id": "AKIAIOSFODNN7EXAMPLE",
      "seed": {
        "secret": "...",
        "ses_smtp_password_v4": "..."
      }
    }
  }
}
```

Seeded values are written into state as given. They are the only thing nivis
takes on trust rather than reading from the provider, which is why the result
says which attributes were seeded: you can tell what was asserted from what was
observed.

Leaving one out is not harmless. The attribute is then absent from state while
your configuration expects a value, so the next plan proposes changing a live
credential.

Not every seed is a secret. Some attributes are create-time instructions that
the object itself does not record, so a read cannot return them either.
`postgresql_schema.if_not_exists` is one: a created schema has it set, a read
after import returns null, and without seeding it the next plan proposes a
change that is not real. The test for this is the one place nivis drives a real
third-party provider, which is how the case was found.

> **A manifest with seeds holds credentials.** Do not commit it. `nivis import`
> says so when it reads one.

## Import refuses rather than guesses

`nivis state push` can already write state documents by hand. Import is worth
using only because it is safer than that, and the safety is in what it refuses.
Each refusal is a case where carrying on would write state that looks right and
is not.

| refusal | why |
|---|---|
| the id is not declared in your configuration | the entry would never be planned, applied or destroyed, so you would believe it was managed |
| the provider returned more than one resource | every nivis address comes from your configuration, so there is nowhere to put the others |
| the object does not exist | the import id identified nothing. Creating it instead could duplicate something that is already there |
| the resource is already in state | importing over it would discard what is there |

A refused entry writes nothing and does not stop the others. A run over two
hundred resources that fails on three tells you which three.

### When a provider returns more than one resource

A few resource types return several objects from one import. Nivis refuses and
names what came back, because it has no addresses to put them at: declare each
resource and import it by its own id, or write their state directly with
`nivis state push`.

## Check the import with a plan

```sh
nivis plan
```

A clean plan is the only honest proof the import was right. If nivis imported
the wrong object, your configuration will not match what state now describes,
and the plan will say so.

This matters most where it is least convenient. An import that looks like it
worked, and whose first apply destroys a production resource, is the failure this
command is built to avoid, and the plan is the check that catches it.

## State written by Terraform

A Terraform state document records the provider schema version each resource was
written at. Nivis records the same thing and asks the provider to upgrade stored
attributes when the configured provider has moved on, so adopting an estate from
an older provider version does not silently reshape it.
