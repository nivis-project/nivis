# Datasources

A **datasource** reads existing infrastructure (an AMI by filter, a VPC, an
availability zone, an existing bucket) and feeds it into your resources. Where a
resource is *created*, a datasource is *read*: Nivis never plans, applies, writes
to state, or destroys it. Declare one with `nivis.mkData` and wire its outputs
into resources exactly as you wire one resource into another.

## Declare a datasource with `mkData`

`mkData` mirrors `mkResource` but for a read:

```nix
nivis.plan =
  ledger:
  let
    ami = lib.mkData {
      provider = "aws";
      type = "aws_ami";
      name = "ubuntu";
      config = {
        most_recent = true;
        owners = [ "099720109477" ];        # canonical
        filter = [ { name = "name"; values = [ "ubuntu/images/*-24.04-*" ]; } ];
      };
    };
  in
  lib.toIR {
    providers.aws = lib.mkProvider {
      source = "registry.opentofu.org/hashicorp/aws";
      config.region = "eu-central-1";
    };
    dataSources = [ ami ];
    resources = [
      (lib.mkResource {
        provider = "aws"; type = "aws_instance"; name = "web";
        config = {
          ami = ami.refAttr "id";           # the datasource output feeds the resource
          instance_type = "t3.micro";
        };
      })
    ];
    inherit ledger;
  };
```

Pass datasources to `toIR` in a `dataSources = [ ... ]` list (distinct from
`resources`). A datasource exposes `refAttr`/`refPath` just like a resource, so
`ami.refAttr "id"` is an ordinary cross-node reference that creates a dependency
edge.

Its id is namespaced `data.<provider>.<type>.<name>` (so it never collides with a
resource id), and it carries no lifecycle: a datasource is read, never created.

## When a datasource is read (the phased model)

Nivis reads a datasource **when its config inputs are fully known**, using the
same per-phase readiness as resources. Two cases:

- **Fully-known config** (the common case): the datasource reads in the **first
  phase**, before the resources that depend on it, and its outputs are available
  immediately.
- **Config that depends on a resource's apply-time output**: the resource applies
  first, then the datasource reads in a **later phase** once that output is known,
  then resources that consume the datasource apply after that.

So a datasource participates in the round trip: you can apply a resource, read a
datasource computed from its output, and feed that back into more resources, all
in one `nivis apply`. A datasource whose config never becomes fully known is
reported as a stuck node, the same as an unresolvable resource.

## How it differs from a resource

| | Resource | Datasource |
| --- | --- | --- |
| Lifecycle | create / update / replace / destroy | read only |
| Appears in `nivis plan` | yes (with a change marker) | no |
| Written to state | yes | no |
| In the dependency graph | yes | yes (refs in and out) |
| Re-read on each run | n/a | yes (no caching) |

## What is not here yet

- **Typed `mkData` constructors from a provider schema** (`nivis gen`). `mkData`
  is hand-usable now; codegen for datasources comes later.
- **Caching / staleness policy.** A datasource is read once per run when ready;
  there is no cross-run cache.
- **`depends_on` on a datasource.** Implicit refs are enough for now.
