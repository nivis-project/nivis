---
# nixform2-l0th
title: nivis never sends ClientCapabilities, and drops the WriteOnly schema flag
status: todo
type: bug
priority: normal
tags:
    - discovered
created_at: 2026-10-15T19:47:50Z
updated_at: 2026-10-15T20:23:41Z
blocking:
    - nixform2-3utj
---

Found while spiking nixform2-jkeh on 2026-10-15 against the real
`cyrilgdn/postgresql` provider.

Write-only attributes are a protocol feature with two halves, both present in
nivis's own generated protobuf for tfprotov5 and tfprotov6:

    ClientCapabilities.WriteOnlyAttributesAllowed   tfplugin5.pb.go:842
    Schema_Attribute.WriteOnly                      tfplugin5.pb.go:2453

The attribute flag's own documentation: "the attribute value will be provided
via configuration and must be omitted from plan and state response objects".

nivis references neither outside the generated code:

- it never sends `ClientCapabilities` on any RPC, so it never opts in;
- it drops the per-attribute `WriteOnly` flag when building its schema
  representation (`internal/provider/*/`, `schemaRaw` carries objType and
  computed only).

## Why this matters

A write-only attribute is set from configuration and must NOT appear in the
planned state or in state. nivis does not know which attributes those are, so it
compares and stores them like any other. The predictable consequences are a
perpetual diff, or a no-op detection that never fires, for any resource using
one.

`postgresql_role` has a live example: `password_wo`, alongside
`password_wo_version` whose purpose is to control when the write-only value is
re-sent.

It also matters that this is a NEGOTIATION. A provider that sees no opt-in may
behave differently from one that does, and nivis has exercised neither path.

## Relation to import

nixform2-3utj requirement 1 ("attributes a provider never returns") describes
exactly what write-only models. Deciding whether import needs a nivis-side
seeding mechanism depends on this: if providers declare these attributes and
nivis honours the declaration, seeding is a second mechanism for a solved
problem. See 3utj for the reasoning.

## Worth checking as part of this

Whether `aws_iam_access_key.secret` and `ses_smtp_password_v4` are declared
write-only by the AWS provider. If they are, 3utj's hardest requirement is
mostly a matter of honouring the protocol. If they are not, seeding may still be
needed for them specifically, and that is a much narrower feature.


## Vehicles for testing this (2026-10-15)

| provider  | write-only                                  |
|-----------|---------------------------------------------|
| postgres  | `password_wo`, confirmed in schema source |
| libvirt   | none: 0 in source, 0 in 12,430 doc lines    |
| vault     | 66 code-search hits                         |
| minio     | 23                                          |
| keycloak  | 13                                          |

The last three are raw code-search counts including tests and non-schema uses,
so they indicate the feature is used rather than how many attributes declare it.
Only postgres is confirmed at the schema level.

Postgres is therefore the vehicle via nixform2-jkeh. If a second is wanted once
the behaviour is understood, vault looks richest and minio was already the
runner-up candidate in nixform2-q29p for its S3 semantics. Verify at the schema
level before relying on either.


## Scope: this does not cover the AWS import case (2026-10-15)

Checked while sizing nixform2-3utj. `aws_iam_access_key.secret` and
`ses_smtp_password_v4` are `Computed: true, Sensitive: true` and NOT
`WriteOnly` (`terraform-provider-aws`,
`internal/service/iam/access_key.go`). `Read` never restores them, and the
provider's own test annotation lists them under `importIgnore`.

So supporting write-only is worth doing for its own reasons (a provider that
declares it expects the client to have opted in, and nivis currently compares
and stores such attributes like any other), but it does not solve 3utj
requirement 1. That case is a different shape: a value the provider produced
once and will never return, with nothing in the schema marking it.


## The harness exists (2026-10-15)

`real-postgres-provider-e2e` landed a nixosTest that drives nivis against the
real `cyrilgdn/postgresql` provider, offline, in 13 seconds (bean
nixform2-jkeh). `tests/vm/postgres.nix` and `nix/example/postgres.nix` are
the shape to extend: adding a resource or an assertion is now a few lines rather
than a new tier.
