---
# nixform2-rlbz
title: S3 backend — configurable server-side encryption (SSE-KMS support)
status: completed
type: task
priority: normal
created_at: 2026-08-31T21:22:26Z
updated_at: 2026-09-24T14:54:14Z
parent: nixform2-kovh
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

## Summary of Changes

OpenSpec change: `configurable-s3-backend-encryption`.

`backend.sseAlgorithm` now selects how the s3 backend encrypts the objects it
writes, with `backend.kmsKeyId` for the KMS case:

- `"AES256"`, the value assumed when the key is absent, so existing
  configurations are untouched.
- `"bucket-default"`, which sends no encryption header at all and lets the
  bucket's own default rule apply.
- `"aws:kms"` with `kmsKeyId`.

Both writes follow the setting, the state object and the `<key>.lock` object,
which matters because the lock is the first S3 call of a mutating run and is
therefore where an enforcing bucket denies first.

### What the investigation changed about the plan

The bean proposed `aws:kms` + `kmsKeyId`. Reading the actual modules
(`terraform-aws-module-terraform-backend` to `terraform-aws-s3`, pinned
`945d79d`) showed that is not the mode TechNative's buckets need. Their policy
denies a write whose encryption header is PRESENT and not the bucket default:

```
A  Deny if  x-amz-server-side-encryption-aws-kms-key-id  present AND != var.kms_key_arn
B  Deny if  x-amz-server-side-encryption                 present AND != "aws:kms"
```

nivis sent `AES256`, statement B matched, and the lock write was denied. Sending
NO header satisfies both statements, which is also what Terraform does there (the
stacks pin `required_version = "< 1.6.0"`, where the backend's `encrypt` defaults
to false). So `"bucket-default"` is the mode that unblocks the real case, and it
is safer than `"aws:kms"` against these buckets: statement A compares the key id
as a string, so an alias or bare key id names the same key and is still denied.
All three modes ship because a policy that denies a MISSING key id needs the
explicit one.

### Also in this change

- An access denial on a write now says a server-side encryption mismatch may be
  the cause and which mode to try, tailored to the configured mode. S3 answers an
  explicit policy Deny with a bare `AccessDenied` that never mentions encryption.
  The hint is appended, never replacing the underlying error, and is not attached
  to reads (a read sends no encryption parameters).
- **Breaking:** an unrecognized `backend` key is now rejected rather than
  ignored, for every backend type. This rode along because this change is what
  made the silent drop dangerous: a typo such as `sseAlgorythm` used to be
  harmless and would now yield `AES256`, a 403, and a hint pointing at the very
  setting the user was looking at. Misspellings suggest the key they resemble
  (comparison ignores case and word separators); Terraform keys carried across
  (`dynamodbTable`, `roleArn`, `encrypt`, `profile`, `sessionName`) say what
  nivis does instead.

### Tests

`internal/fakes3` gained the KMS key id header, `DenyMismatchedSSE` (statement B)
and `DenyMissingSSE` (the other shape), so the acceptance asserts that a run is
ACCEPTED rather than that nivis sent a particular header. The e2e case runs the
real binaries against an enforcing bucket: denied under the default, accepted
under `"bucket-default"`. Back-compat is pinned by a test asserting `AES256` on
both objects when no new key is declared.

### Not in this change

Assume-role for state access stays `nixform2-nd2c`. Note that it now has a small
coupling: the strict key set rejects `roleArn` with "not supported yet", so that
bean must add `roleArn`/`assumeRole` to the accepted set and drop that message.
