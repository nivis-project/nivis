---
# nixform2-7xxn
title: error improvement s3 remote state credentials expired
status: draft
type: feature
priority: normal
created_at: 2026-09-10T20:48:18Z
updated_at: 2026-09-10T20:49:52Z
---


# nivis error and log output

Status: design agreed, not yet implemented.
Scope: how nivis renders errors and info logs on a terminal and in CI.

## Problem

Errors currently reach stderr as the raw wrapped chain:

```
error: state: s3: acquire lock nivis-demos-state-a0dc640ed260/030_vaultwarden_hetzner/state.json.lock: operation error S3: PutObject, get identity: get credentials: failed to refresh cached credentials, operation error STS: AssumeRole, https response error StatusCode: 403, RequestID: 80e2ad54-2954-4f97-9959-95afcb077412, api error ExpiredToken: The security token included in the request is expired
```

Three things are wrong with it:

1. The chain reads outermost-first, so the root cause (`ExpiredToken`) is last. Humans need the opposite order.
2. Identifiers (bucket, key, request ID, status) are embedded in prose, so the eye has to parse English to find them.
3. There is no remediation. The user has to know that `ExpiredToken` means "run `aws sso login`".

Non-goal: shortening the output. Every fact in the raw string must survive somewhere — the default view, `-v`, or the JSON stream. Losing the request ID makes AWS support tickets unanswerable.

## Target output — default

This is a golden fixture. The renderer must produce this byte-for-byte with color disabled.

```
✗  Cannot acquire state lock

   Your AWS credentials have expired.
   Refresh them and re-run:  aws sso login --profile nivis-demos

   state       030_vaultwarden_hetzner
   bucket      nivis-demos-state-a0dc640ed260
   lock key    030_vaultwarden_hetzner/state.json.lock
   call        s3:PutObject → sts:AssumeRole
   response    403 ExpiredToken
   request id  80e2ad54-2954-4f97-9959-95afcb077412

   run with -v for the full error chain
```

Layout rules:

- Headline is what the *user* was trying to do, in nivis vocabulary, not the SDK's operation name.
- Cause is one sentence, plain language, second person ("your credentials"), no error code in it.
- Fix is a runnable command where one exists; otherwise an imperative step. Never both a command and a paragraph.
- Field labels are lowercase, left-aligned, padded to the longest label + 2 spaces.
- Omit a field entirely when its value is unknown. Do not print empty values in the default view.
- Blank line between headline, remediation, fields, and footer.

## Target output — `-v`

`-v` must add facts the default view withheld. Re-rendering the same six facts in a different shape is not verbose output. Test for each line: *could this line change what the user does next?*

Everything from the default view, then:

```
   identity
     profile     nivis-demos (from AWS_PROFILE)
     source      sso cache ~/.aws/sso/cache/9f2c8a1e.json
     role        arn:aws:iam::730335249114:role/nivis-deployer
     expired     2026-09-10 09:14:02Z (23m ago)

   request
     region      eu-central-1 (from ~/.aws/config)
     endpoint    https://nivis-demos-state-a0dc640ed260.s3.eu-central-1.amazonaws.com
     attempts    3 of 3 (backoff 1.2s, 2.4s — all 403)
     elapsed     4.81s
     host id     8vT0nQmzKp3xR1a/dW+bYh4LcO9…= (x-amz-id-2)
     clock skew  +0.4s (server 09:37:18Z)

   lock
     held by     unknown — read of .lock also failed
     state age   unknown

   raw
     state: s3: acquire lock nivis-demos-state-a0dc640ed260/030_vaultwarden…
```

Why each field earns its place:

| Field | Distinguishes |
|---|---|
| `expired` + delta | session lapsed vs. token never valid |
| `role` | wrong profile assumed vs. missing IAM permission |
| `source` | which credential file to go fix |
| `attempts` | consistent rejection vs. flaky endpoint |
| `clock skew` | rules out signature failures from a drifting clock |
| `host id` | AWS support requires it alongside the request ID for S3 |

Under `-v`, print fields whose value could not be determined as `unknown` with a reason. Negative results are information: "held by unknown — read of .lock also failed" stops the user hunting for a stale lock. This is the one place the omit-empty rule is inverted.

## Verbosity levels

| Level | Contains |
|---|---|
| default | cause, fix, 6 identifiers |
| `-v` | + identity resolution, request metadata, timing, raw error string |
| `-vv` | + each retry attempt separately, redacted request/response headers, Go error chain annotated with concrete types and the wrapping package |
| `--log-format=json` | all of the above, one JSON object per event |

The Go error chain belongs at `-vv` and only when annotated with types (`*state.LockError → *awshttp.ResponseError → *types.ExpiredToken`), because the type names tell the reader which file to open. An unannotated chain is the same sentence with line breaks and must not be printed.

## Types

New package `internal/diag`.

```go
type FieldKind int

const (
	KindPlain FieldKind = iota
	KindResource // bucket, key, ARN, path, stack name
	KindStatus   // HTTP status, exit code
	KindCode     // API error code
	KindCommand  // runnable shell command
	KindDim      // request IDs, timestamps, provenance notes
)

type Field struct {
	Label string
	Value string
	Kind  FieldKind
	Group string // "" for the default block, else "identity", "request", "lock", "raw"
}

type Diagnostic struct {
	Summary string   // "Cannot acquire state lock"
	Cause   string   // "Your AWS credentials have expired."
	Fix     []string // commands or imperative steps
	Fields  []Field  // ordered; groups render in declaration order
	Err     error    // untouched original, for -v/-vv and JSON
}
```

Errors must arrive at the renderer as typed values carrying fields, not as concatenated strings. Every wrap site that knows something the renderer needs gets a type:

```go
type LockError struct {
	Backend string // "s3"
	Bucket  string
	Key     string
	Stack   string
	err     error
}

func (e *LockError) Error() string { return "acquire lock " + e.Bucket + "/" + e.Key }
func (e *LockError) Unwrap() error { return e.err }
```

## Classification

`diag.Classify(err) *Diagnostic` walks the chain with `errors.As`. The innermost match wins, because AWS puts the root cause deepest.

Remediation is a lookup table on the API error code, kept in one place so it accumulates:

| Code | Cause | Fix |
|---|---|---|
| `ExpiredToken`, `ExpiredTokenException` | Your AWS credentials have expired. | `aws sso login --profile $AWS_PROFILE` |
| `AccessDenied`, `AccessDeniedException` | The assumed role is not allowed to perform this call. | `aws sts get-caller-identity`; check bucket policy and role policy |
| `NoSuchBucket` | The state bucket does not exist in this region. | verify `AWS_REGION` matches the bucket's region |
| `InvalidClientTokenId` | The access key is not valid for this account. | check `AWS_PROFILE` and `~/.aws/credentials` |
| `RequestTimeTooSkewed` | Your system clock is too far from AWS time. | sync the clock (`timedatectl`, `w32tm /resync`) |

Unknown codes fall back to a summary of "Command failed", the code as `response`, and no `Fix`. Never invent remediation for a code not in the table.

## Color

Color is a property of `FieldKind`, never of a call site. One theme maps kinds to ANSI so a later restyle is a single edit.

| Kind | ANSI |
|---|---|
| headline / `KindStatus` | bright red |
| `KindCode` | yellow |
| `KindCommand` | green |
| `KindResource` | bright blue |
| `KindDim`, labels | bright black |
| `KindPlain` | default |

Resolve once at startup: enable color only when `--color` is `always`, or when it is `auto` and `NO_COLOR` is unset and `TERM != "dumb"` and `term.IsTerminal(int(os.Stderr.Fd()))`. CI logs full of escape sequences are worse than no color.

Diagnostics go to **stderr**. stdout stays clean so `nivis output -o json | jq` keeps working.

## Attempt middleware

Build this first — every nivis AWS call gets timing and skew for free once it exists, and three `-v` fields depend on it.

A smithy middleware in the `Finalize` step, recording into a per-operation struct on the context:

- each attempt: start time, HTTP status, error code, backoff before it
- total elapsed
- the response `Date` header, and local time at receipt, for skew
- for S3, the `x-amz-id-2` header (`*s3.ResponseError` exposes `HostID()`)

Credential provenance (`profile`, `source`, `region`, expiry) is captured separately at `config.LoadDefaultConfig` time and stashed on the app context. On a refresh failure you generally cannot get fresh credentials, so read the cached expiry rather than re-invoking the provider.

## Acceptance criteria

1. Golden-file tests for the default and `-v` renderings above, with color forced off, matching byte-for-byte.
2. A test asserting `NO_COLOR=1` and a non-TTY stderr both produce zero escape sequences.
3. A test asserting the request ID, host ID and status code appear in `--log-format=json` output even when the default view omits them.
4. A test that an unrecognised API error code renders without a `Fix` block and without a fabricated cause.
5. `go vet` clean; no `fmt.Errorf` writing directly to stderr anywhere outside `internal/diag`.

## Implementation order

1. `internal/diag`: types, renderer, theme, color detection, golden tests.
2. Attempt middleware and the app-context struct it feeds.
3. `LockError` and the S3 state backend wrap sites; wire `diag.Classify` into the top-level error handler in `main`.
4. Remediation table, extended per code as they show up in practice.
5. `slog` handler sharing the same `Field` vocabulary, so info logs and errors look like one tool. Human-readable on a TTY, JSON otherwise.
