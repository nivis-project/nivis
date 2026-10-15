---
# nixform2-3utj
title: import state, just like terraform can do
status: draft
type: feature
priority: normal
created_at: 2026-09-10T20:18:27Z
updated_at: 2026-10-15T21:30:37Z
---

Requirements gathered from a real migration: AWS account 255418484322 (Waardenburg), 200 Terraform resource instances that have to move into nivis without being recreated. Two live websites behind Cognito, a delegated Route53 zone, and SES with working SMTP credentials. Recreating any of it is not an option, so import is the gate for that whole migration.

## State of the plumbing

`ImportResourceState` exists as a generated gRPC client method in `internal/tfplugin5` and `internal/tfplugin6`, and the fake providers answer it with `unimplemented`. Nothing in nivis calls it. So this is adding a caller and a command, not implementing a protocol.

`UpgradeResourceState` is already called, which matters here: imported state may come from a different provider schema version than the one in use.

## Requirements

### 1. Attributes a provider never returns

The hard case, and the one that makes a naive import dangerous.

Some attributes exist only at creation. `aws_iam_access_key` returns `secret` and `ses_smtp_password_v4` once, when created, and never again. `ReadResource` and `ImportResourceState` both come back without them.

If the state says such an attribute is absent while the configuration expects a value, the next plan proposes a replacement. For an access key that means rotating a live credential: in this account that would break the `linkchecker` SMTP user and the deploy user of a running website.

Import must therefore accept seeded attribute values that are written into state as given, without being read back from the provider. Terraform has the same problem and handles it poorly, which is worth reading as a warning rather than a model.

Evidence in this account: three `aws_iam_access_key` resources, all with `secret` and `ses_smtp_password_v4` present in the Terraform state document.

### 2. Identity is not always the `id` attribute

A Route53 zone is addressed by `Z0042665NOVUJHACYP2Q`, an S3 bucket by its name, an IAM user by its name, a CloudFront distribution by its id.

`ImportResourceState` takes an opaque import string and lets the provider decide what it means, so nivis needs no per-type knowledge. Worth stating explicitly because it is the reason to use the protocol method rather than hand-building state.

### 3. Bulk, not one call per resource

Two hundred resource instances in this one account, and that is a small account. A CLI invocation per resource is unworkable.

The usable shape is a manifest mapping nivis resource id to provider import id, applied in one run, with a report of what succeeded and what did not. Partial failure has to leave the state coherent.

### 4. The acceptance test is a clean plan

After importing, `plan` must report no changes. That is the only honest proof the import was correct.

Stronger: a wrong import must not be able to quietly propose a destroy or replace on live infrastructure. Whatever the implementation, the dangerous outcome is an import that looks like it worked and whose first apply deletes a production CloudFront distribution. Prefer refusing over guessing.

### 5. Relation to what already exists

`state push` plus `refresh` can already do this today with hand-written state documents. The format is simple: `{"resources": {"<id>": {"id", "type", "attrs"}}}`.

So import is comfort and safety on top of an existing capability, not a new one. That is not an argument against building it, but it does set the bar: it has to be meaningfully safer than hand-editing state, or it adds nothing. Requirements 1 and 4 are where that safety lives.

## Open questions

- Does `refresh` populate attributes correctly from a minimal `attrs` containing only the identity? If so, much of this can be prototyped against `state push` before any new command exists.
- What should happen when a resource exists in the provider but its configuration differs from reality? Import, then let plan show the drift, seems right, but it means an import can be followed by a destructive plan that is nonetheless correct.


## Pre-implementation findings (2026-10-11)

### Correction: UpgradeResourceState is NOT called

The "State of the plumbing" section above is wrong on this. Only the three
FAKES implement it as servers. Nothing in nivis calls it, and it is not on the
`provider.Client` interface (`internal/provider/provider.go`).

Worse, `state.ResourceState` is `{id, type, attrs}` with no schema version
recorded, and `UpgradeResourceState` needs a version to upgrade FROM. So
importing a Terraform state document written at an older provider schema has no
upgrade path, and nowhere to put the version. That is a state-format decision,
so it cannot be retrofitted quietly. It belongs in the proposal.

### The substrate cannot express import

- `ImportResourceState`: unimplemented in all three fakes.
- `ReadResource`: all three return `req.CurrentState` verbatim.

The second blocks this bean's own first open question. A "state push minimal
attrs + refresh" prototype would appear to FAIL against the fakes and appear to
WORK against AWS, for reasons unrelated to nivis. Answering it needs either a
fake that models read-populates-from-identity, or a credentialed run.

### A hazard in the shape of requirement 4

`internal/phase/driver.go`, `priorState`: when a refresh read returns no
attributes it is treated as an out-of-band deletion, so prior is nil and the
resource is planned as a CREATE. Import a resource with a wrong id, or hit a
provider returning empty for an id it does not recognise, and the next plan
proposes creating something that already exists. Nothing distinguishes "this
was deleted" from "I could not find what you imported". Per this bean's own
"prefer refusing over guessing", an unresolvable import should refuse rather
than fall through to that path.

### Order of work before implementing

1. Correct the premise above and decide where a schema version lives in state.
2. Substrate: fakes serve `ImportResourceState`, and a read that populates
   from identity.
3. Settle the clean-plan assertion mechanically. `PlanReport` already returns
   ops, so "every op is NoOp" is assertable today; that test should exist
   before the importer does.
4. Decide what an unresolvable import does.

Then requirements 1 and 4 can be prototyped through `state push` + `refresh`
+ `plan` with no new command, which is what tells you whether import clears
this bean's own bar ("meaningfully safer than hand-editing state, or it adds
nothing").

## Open beans worth closing first

Not wired as hard blockers; this is the reasoning, the scheduling is yours.

| bean | why it matters HERE |
|---------------|--------------------------------------------------------|
| nixform2-j02d | `PlanReport` seeds the ledger with `if stored, err := d.Store.List(); err == nil`. A failed state read leaves the ledger empty, so every resource reports as a CREATE. For import verification that is the worst failure: the plan looks like the import did nothing, and applying would duplicate 200 live resources. Requirement 4 rests on this. |
| nixform2-gmmo | A possible perpetual diff from a derived leaf fed by a datasource. Requirement 4 is "plan reports no changes"; if this is real, clean-plan fails for reasons unrelated to import. Recorded as an observation, so confirm or kill it before it becomes the gate. |
| nixform2-znyj | "A misreading produces a fake that agrees with the misreading, a green suite, and a broken production path." Import cannot be proven against fakes at all, and this bean is written from a real account with 200 resources that must not be recreated. This decision shapes HOW import is built, not just how it is tested. |
| nixform2-8btx | Import is warning-heavy (deprecated attributes, "import did not set X"). Requirement 1 is about attributes a provider will not return, and it may say so in a diagnostic nivis currently drops. Can ride alongside the import work rather than before it. |

Suggested order: j02d, then gmmo confirm-or-kill, then decide znyj.

Not relevant to import: nwnf, vh2s, n7h6, p4uz, ih51, 7xxn. Open but unrelated
and cheap: rkn4 (high), and the gate cluster pex6 / w74f / 455o / p72l.


## Testing tier (2026-10-15)

nixform2-q29p adds a hermetic real-provider tier. `cyrilgdn/postgresql` is in
it specifically for requirement 1 above: `postgresql_role.password` is an
attribute the provider never returns, verified in its source. That makes the
hard case of this bean reproducible in a VM in seconds, instead of only against
three live `aws_iam_access_key` resources in a production account.


## Test vehicle per requirement (2026-10-15)

Supersedes the single-line note above. All verified in provider source, not
assumed. Substrate is nixform2-q29p.

| requirement                               | vehicle                      |
|-------------------------------------------|------------------------------|
| 1. attributes a provider never returns    | `postgresql_role.password`: Sensitive, `Read` never sets it. Import a role and the plan is NOT clean. That is this requirement's failure, hermetic, in seconds |
| 2. identity is not the id attribute       | postgres imports by role or database NAME; libvirt by UUID or path. Both are the natural identity, not a synthetic id |
| 3. bulk, not one call per resource        | create 50 roles out of band with SQL, import via a manifest, assert one clean plan |
| 4. acceptance is a clean plan             | `PlanReport` returns ops; "every op is NoOp" is assertable today |

Both providers implement import:
`postgresql_role` and `postgresql_database` use
`schema.ImportStatePassthroughContext`; libvirt has `ImportState` on domain,
volume, network and pool.

### Open question 1 is answerable without AWS

`ImportStatePassthroughContext` sets the id from the import string and relies
on READ to populate the rest. So "does refresh populate attributes correctly
from a minimal attrs containing only the identity?" is exactly what postgres
does, and its Read queries `pg_roles` by name. No credentialed run needed to
answer it.

### A protocol feature to check before building

`postgresql_role` declares `password_wo` with `WriteOnly: true`, and
`password_wo_version` to control when it re-applies. Write-only attributes are
a protocol-level feature and there is no evidence nivis handles them. They are
also the provider ecosystem's own answer to requirement 1, so they are worth
understanding BEFORE designing a seeding mechanism: the right design may be to
support write-only attributes rather than to invent seeding.

### The schema-version hole is not closed by this substrate

`resource_postgresql_role.go` has no `SchemaVersion` and no
`StateUpgraders`, so neither provider exercises the state-upgrade path.
Combined with the finding above (nivis never calls `UpgradeResourceState`,
`state.ResourceState` records no version), importing a Terraform state written
at an older provider schema stays unexercised. It needs a provider with a
version bump, or it is a known hole to state plainly in the proposal.


## Acceptance test prerequisite closed (2026-10-15)

nixform2-j02d is fixed (commit 76779d7). Requirement 4 above ("after importing,
plan must report no changes") now rests on a plan that cannot silently report
every resource as a create when the state backend is unreadable. Before this, a
denied credential during import verification would have looked exactly like an
import that did nothing.


## The write-only question is answered (2026-10-15)

Spiked against the real `cyrilgdn/postgresql` provider via nixform2-jkeh.

**The protocol already solves requirement 1, and nivis ignores it.**

- `ClientCapabilities.WriteOnlyAttributesAllowed` (`tfplugin5.pb.go:842`):
  the client opts in. nivis never sends `ClientCapabilities` at all.
- `Schema_Attribute.WriteOnly` (`tfplugin5.pb.go:2453`): "the attribute
  value will be provided via configuration and must be omitted from plan and
  state response objects". nivis drops the flag.

Both fields are in nivis's own generated protobuf for v5 and v6. Neither is
referenced anywhere else in the codebase.

So the design fork in requirement 1 (invent nivis-side seeding, versus support
the protocol feature) resolves toward the protocol: an attribute that must be
provided by configuration and omitted from state is EXACTLY what requirement 1
describes, and providers already declare it. `postgresql_role.password_wo` is
a live example.

Seeding would be a second, parallel mechanism for a problem the protocol already
models, and it would not help a provider that declares write-only attributes
and expects the client to have opted in.

What seeding might still be needed for: an attribute that is NOT declared
write-only but is nonetheless never returned, which is the
`aws_iam_access_key.secret` case. Whether AWS declares it write-only is worth
checking before deciding, and is now a concrete question rather than a design
preference.


## Correction: write-only does NOT cover the AWS case (2026-10-15)

The section above concluded that requirement 1 "resolves toward the protocol"
and that seeding would be a second mechanism for a solved problem. That is right
for postgres's `password_wo` and WRONG for the case this bean was written
from.

`terraform-provider-aws`, `internal/service/iam/access_key.go`:

    "secret":               { Type: TypeString, Computed: true, Sensitive: true },
    "ses_smtp_password_v4": { Type: TypeString, Computed: true, Sensitive: true },

Computed and Sensitive, NOT WriteOnly. `Read` sets only `create_date`,
`status` and `user`; it never restores either secret.

The provider's own test annotation at line 36 admits the problem:

    // @Testing(importIgnore="...;secret;ses_smtp_password_v4;encrypted_ses_smtp_password_v4")

AWS's own test framework special-cases these on import because they do not come
back.

### Two different shapes, and only one is in the protocol

| | write-only (`password_wo`) | computed-and-gone (`secret`) |
|---|---|---|
| value comes from | configuration | the provider, once, at create |
| declared in schema | yes, `WriteOnly` | nothing marks it |
| after an import | correctly absent from state | silently absent, nothing says why |

Supporting write-only (nixform2-l0th) remains worth doing on its own merits, but
it would not fix importing an `aws_iam_access_key`. Nothing in the schema tells
a client that `secret` can never be read back: the information is not in the
protocol at all.

So seeding, or something like it, IS needed for requirement 1. The narrower
question that remains is whether nivis can distinguish the two cases
automatically, or whether the manifest has to say which attributes are seeded.
Given this bean's own "prefer refusing over guessing", the manifest saying so
explicitly looks right.

Cost note: answering this needed no credentials. A provider schema is readable
without configuring it, and in this case the answer was in the provider source.


## The harness exists (2026-10-15)

`real-postgres-provider-e2e` landed a nixosTest that drives nivis against the
real `cyrilgdn/postgresql` provider, offline, in 13 seconds (bean
nixform2-jkeh). `tests/vm/postgres.nix` and `nix/example/postgres.nix` are
the shape to extend: adding a resource or an assertion is now a few lines rather
than a new tier.


## The schema-version question is proposed (2026-10-15)

It is a bug in its own right, not an import gap: a provider upgrade silently
mangles stored state today, because nivis drops `Schema.Version` and never
calls `UpgradeResourceState`. Filed as nixform2-67gg (high), proposed as
`honour-provider-schema-versions`, and it blocks this bean.

Once it lands, an imported Terraform state document can carry the
`schema_version` it came with, instead of being assumed to match whatever
provider happens to be configured. That assumption is exactly the "looks like
it worked, first apply destroys production" failure this bean exists to
prevent.


## Every open question answered (2026-10-15)

### The two in this bean

**"Does refresh populate attributes correctly from a minimal attrs containing
only the identity?"** It must, by construction, because that IS how import
works. `schema.ImportStatePassthroughContext` is literally
`return []*ResourceData{d}, nil`: only the ID is set. OpenTofu then refreshes
(`node_resource_import.go:269`, `riNode.refresh(...)`). So the flow is
import -> stub carrying little more than the id -> read -> full attributes ->
state. A provider that cannot populate from the identity has broken import, not
nivis.

**"What should happen when a resource exists but its configuration differs from
reality?"** Import reflects reality, and the next plan shows the drift. That is
correct, and it is what this bean suspected. The protection against a WRONG
import is not drift handling but an existence check, below.

### Four more the exploration surfaced

**An undeclared id must be refused.** Both `PlanReport` and `destroy.Run`
iterate the CONFIG graph (`g.Order`, `dag.DestroyOrder()` with a
`g.Nodes[id]` lookup that `continue`s on a miss). So a state entry whose id
is not in the configuration is never planned, never applied, never destroyed. It
appears in `nivis state list` and nowhere else. Writing one would be "an
import that looks like it worked" in its quietest form: the user believes it is
managed and nothing will ever touch it.

**A multi-resource response must be refused.** `ImportResourceState` returns a
LIST of `ImportedResource`, each with its own `TypeName`. OpenTofu invents
addresses for the extras (overriding the type, suffixing the name `-1`,
`-2`). nivis cannot: an invented id is not in the Nix config, so by the
paragraph above it would be inert the moment it was written. Copying Terraform
here would manufacture the exact silent failure requirement 4 exists to prevent.

**Seeded attributes go AFTER the refresh.** Forced, not chosen: the refresh
returns the provider's full view, which by definition excludes the
never-returned attributes, so seeding first would be overwritten.

**A non-existent object must be refused, NOT treated as a create.** OpenTofu
checks after the refresh:

    if state.Value.IsNull() {
        "Cannot import non-existent remote object ... Only pre-existing
         objects can be imported; check that the id is correct"
    }

This collides with nivis's existing behaviour. `Driver.priorState` turns an
empty read into "deleted out of band" and plans a CREATE. Import must not
reuse that path: for an import, an empty read means the id was wrong, and
creating a duplicate of something that may well exist is the destructive
outcome this bean is written against.

### Requirement 3 is simpler than feared

"Partial failure has to leave the state coherent" needs no machinery. Each
import either writes one complete entry or none, so there is no partial entry to
be coherent about. The successful ones stay imported; the failed ones are
absent.

### Prerequisites now all closed

- nixform2-j02d: a plan can no longer silently report creates when the backend
  is unreadable, so requirement 4's acceptance test is trustworthy.
- nixform2-67gg: a schema version is recorded, so an imported Terraform document
  can carry the `schema_version` it came with. NOTE nixform2-ff03: `nivis
  refresh` still skips the upgrade and drops the version, and import's flow
  ends in a refresh, so that bean should land first or import inherits the gap.
- nixform2-jkeh: the harness exists for testing all of it.
