---
# nixform2-nd2c
title: S3 backend — assume-role support (parity with the OpenTofu backend)
status: todo
type: task
priority: normal
created_at: 2026-09-02T13:03:06Z
updated_at: 2026-09-02T13:03:06Z
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
