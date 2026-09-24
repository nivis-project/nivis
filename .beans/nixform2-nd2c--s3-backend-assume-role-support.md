---
# nixform2-nd2c
title: S3 backend — assume-role support (parity with the OpenTofu backend)
status: completed
type: task
priority: normal
created_at: 2026-09-02T13:03:06Z
updated_at: 2026-09-24T15:36:50Z
parent: nixform2-kovh
---

The S3 state backend resolves credentials only via the AWS SDK default chain
(`internal/state/s3.go:46`: `LoadDefaultConfig`); the backend block accepts
only `bucket`/`key`/`region`/`endpoint`. There is no way to declare a role to
assume for state access — unlike OpenTofu's S3 backend (`role_arn` /
`assume_role`), which landing-zone setups rely on.

## Why this matters (real-world case)

TechNative's landing zone: operators authenticate as a management-account user
(`AWS_PROFILE=technative`, account 5214...) and every workload account is
reached by assuming `landing_zone_devops_*` roles. OpenTofu stacks declare the
switching IN THE REPO: backend `role_arn` in the .tfbackend file, provider
`assume_role` in provider.tf — a new operator needs only the base profile.

nivis supports the provider half already (mkProvider passes `assume_role`
through to the AWS provider). But because the BACKEND can't assume a role, a
member-account state bucket is unreachable from the base profile, and the
whole domain must run under a pre-assumed profile
(`AWS_PROFILE=technative-web_dns`, an assume-role profile in ~/.aws/config).
Works, but moves the role mapping out of the repo into per-operator dotfiles.

## Proposal

Optional backend keys, mirroring OpenTofu:

```nix
backend = {
  type = "s3";
  bucket = "..."; key = "..."; region = "eu-central-1";
  assumeRole = {
    roleArn = "arn:aws:iam::123456789012:role/landing_zone_devops_user";
    sessionName = "nivis-state";   # optional
  };
};
```

Implementation: wrap the loaded config's credentials with
`stscreds.NewAssumeRoleProvider` when `assumeRole` is set (aws-sdk-go-v2
`credentials/stscreds`), in `newS3Store`. Backend config stays static —
consistent with the existing contract.

## Acceptance

- Base-profile operator + backend assumeRole + provider assume_role = full
  repo-declared role switching (OpenTofu parity), verified against a
  member-account bucket.
- Omitting `assumeRole` keeps today's default-chain behaviour.
- docs/REMOTE-STATE.md documents the keys.

Related: nixform2-rlbz (SSE-KMS) — both are "the backend block is thinner than
OpenTofu's"; consider implementing together.

## OpenSpec

OpenSpec change: `s3-backend-assume-role` (spec-driven). Planning artifacts are
complete and validate; implementation not started.

Three things the exploration settled that differ from the proposal above:

- **`externalId` joins `roleArn` and `sessionName`** in the first cut. Third-party
  and cross-account trust policies routinely require it, and without it those
  setups cannot use the feature at all. OpenTofu's remaining `assume_role` fields
  stay out.
- **`profile` stays rejected**, with the reason written into the spec so it does
  not get relitigated: a role ARN resolves the same from any machine, a profile
  name points into one operator's `~/.aws/config` and is commonly absent in CI.
  TechNative's own `RUNME.sh` already makes this call, writing `role_arn` into the
  `.tfbackend` but never `profile`, though Terraform supports one.
- **The change also fixes two things `nixform2-rlbz` left behind.** The backend
  key validator only walks the top level, so a nested block would reopen the
  silent-drop bug that change closed; it becomes a tree walk. And the
  access-denied hint claims a denial is "not a credentials problem", which stops
  being true once nivis resolves credentials of its own.

Related: `nixform2-7xxn` (error and log rendering). Its worked example is already
an `STS: AssumeRole ... ExpiredToken` surfacing through an S3 lock acquire, from
the profile-based workaround. After this change nivis owns that STS call and so
owns that error surface. Either order works; recorded as the change's only open
question.

## Summary of Changes

OpenSpec change: `s3-backend-assume-role`.

The s3 backend takes an optional `assumeRole` block with `roleArn` (required when
the block is present), `sessionName` (defaulting to `nivis`) and `externalId`.
When set, the backend assumes that role and every S3 call it makes uses the
resulting credentials; the default chain stays the source identity, so an operator
still needs their base profile but no longer needs a per-account assume-role
profile in `~/.aws/config`. Credentials sit behind an `aws.CredentialsCache`, so a
run assumes once rather than once per request and survives a session ageing out
mid-apply.

### Two things this had to fix in `nixform2-rlbz`'s wake

- **Key validation only walked the top level.** A nested block would have reopened
  the silent-drop bug that change closed: `assumeRole.rolArn` would have been
  dropped, no role assumed, and the run would fail with an access denial pointing
  nowhere near the cause. `backendKeys` is now a tree, validation recurses into
  declared blocks only, errors name the full path, and suggestions come from the
  block the key sits in. A role setting written at the TOP level (as OpenTofu's
  backend and the `.tfbackend` files allow) is now told which block it belongs in,
  which replaces the two "not supported yet" messages.
- **The access-denied hint claimed the cause was "not a credentials problem".**
  That stopped being true the moment nivis resolved credentials of its own. The
  claim is gone, and when a role is configured the hint names it alongside the
  encryption mode.

### `profile` stays out, deliberately

A role ARN means the same thing from any machine; a profile name points into one
operator's shared config and is usually absent in CI. The reason is written into
the spec and `docs/REMOTE-STATE.md` so it does not get relitigated. The IR
contract's "credentials never in backend" rule was restated to match: no secrets,
an identity MAY be named, no machine-local selectors.

### Testing

A new `internal/fakests` answers `AssumeRole` and records the role, session name
and external id; `internal/fakes3` gained `AllowOnlyAccessKey`, which gates a
bucket on the SigV4 identity and is what lets a test model a member-account bucket
the base identity cannot reach. The e2e runs a full apply against exactly that:
refused without `assumeRole`, accepted with it, state readable back through the
assumed role, and a refused assumption failing legibly.

One test asserts the STS **call count** rather than behaviour, because a missing
credentials cache passes every functional test while making an STS call per
request.

### Still manual

The acceptance is proven hermetically, which covers the mechanism. Confirming it
against a real member-account bucket in the landing zone remains a manual step,
since it needs credentials the suite must not hold. That is an addition to the
automated acceptance, not a substitute for it, and it is the kind of gap a private
live-test setup would close.
