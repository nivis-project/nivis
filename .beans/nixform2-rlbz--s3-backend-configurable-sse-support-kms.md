---
# nixform2-rlbz
title: S3 backend — configurable server-side encryption (SSE-KMS support)
status: todo
type: task
priority: normal
created_at: 2026-08-31T21:22:26Z
updated_at: 2026-08-31T21:22:26Z
---

The S3 state backend hardcodes `ServerSideEncryption: AES256` (SSE-S3) on every
write. There is no way to request SSE-KMS or select a KMS key.

`internal/state/s3.go`:
- line 99  — state object `PutObject`
- line 187 — lock object `PutObject` (`<key>.lock`)

## Why this matters

Enterprise landing-zone buckets commonly enforce SSE-KMS with a specific CMK via
a bucket policy that **explicitly denies** any write that isn't KMS-encrypted
with that key, e.g.:

```json
{ "Effect": "Deny", "Principal": "*", "Action": "s3:*",
  "Condition": {
    "Null": { "s3:x-amz-server-side-encryption": "false" },
    "StringNotEquals": { "s3:x-amz-server-side-encryption": "aws:kms" } } }
```

Against such a bucket, nivis's AES256 `PutObject` gets `403 AccessDenied
(explicit deny in a resource-based policy)` — and because the *lock* write fails
first, `plan`/`apply` can't even start.

## Real-world trigger

TechNative's `web-dns` account (393573040164) reuses one hardened S3 bucket for
all Terraform/OpenTofu state; its policy mandates SSE-KMS with a fixed key.
nivis could not share that bucket and had to provision a **dedicated** state
bucket with AES256 just to function. That defeats "reuse the existing backend"
and leaves nivis state under SSE-S3 while everything else in the account is CMK-
encrypted (the state contains secrets, e.g. a GitHub token in Amplify config).

## Proposal

Make backend encryption configurable in the `backend` block, defaulting to the
current behaviour (back-compat):

```nix
backend = {
  type = "s3";
  bucket = "...";
  key = "...";
  region = "eu-central-1";
  sseAlgorithm = "aws:kms";                 # optional; default "AES256"
  kmsKeyId = "arn:aws:kms:...:key/...";     # required when sseAlgorithm=aws:kms
};
```

- Thread the config through to **both** `PutObject` calls (state + lock): set
  `ServerSideEncryption` and, for KMS, `SSEKMSKeyId`.
- Validate: `kmsKeyId` required iff `sseAlgorithm == "aws:kms"`.
- IR/schema: extend the backend block; keep it static (no refs), consistent with
  the current backend contract.

## Acceptance

- Can `plan`/`apply`/lock against a bucket whose policy denies non-KMS writes.
- Omitting the new keys still writes AES256 (existing behaviour unchanged).
- `docs/REMOTE-STATE.md` "Encryption" section documents the options.
- Tests: `internal/state/s3_test.go` currently asserts `AES256`; add coverage
  for the KMS path (assert `SSEFor` / `SSEKMSKeyId`).
