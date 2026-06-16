# Proposal: nested-block-errors

## Why
AWS (and many providers) model nested blocks as **list-nested**: a block like
`default_tags`, `disk_container`, `user_bucket`, `ingress`, or `ebs_block_device`
is typed as `tftypes.List[Object{...}]`, so in Nivis config it must be written as
a one-element list, `[ { ... } ]`, not a bare attrset `{ ... }`. Writing the bare
attrset fails, but only at apply, with a codec error that leaks type jargon:

```
expected array for tftypes.List[tftypes.Object[...]], got map
```

This has bitten real users twice already: `aws_s3_bucket` `default_tags`
(beans-5ifi) and `aws_ebs_snapshot_import` `disk_container` / `user_bucket`
(during the EC2 tutorial, rx5h). The mistake is easy to make, the error does not
say what to do, and it surfaces only against a real provider at apply time. This
is exactly the kind of cryptic-trap papercut that keeps Nivis from being a
daily-driver (beans-krwc, milestone "Road to v1").

The codec already threads the offending attribute name (`mapToValue` wraps
per-key errors as `["disk_container"]: ...`), so the only gap is that the
innermost message is jargon instead of guidance.

## What changes
- **Actionable error for the list-nested-block-given-an-attrset mistake.** When
  the codec is asked to build a `List` or `Set` (or `Tuple`) value and receives a
  `map` (the decoded shape of a Nix attrset), it SHALL produce an error that
  tells the user to wrap the attrset in a one-element list, instead of the
  raw `expected array for tftypes.List[...], got map`. The message names the
  expected element shape so the fix is obvious, e.g.:
  ```
  ["disk_container"]: this is a list-nested block; wrap the value in a
  one-element list: [ { ... } ]  (got an attrset)
  ```
- **The symmetric mistake too.** When the codec is asked to build a single-nested
  `Object` (or a `Map`) and receives a `[]interface{}` (a Nix list), it SHALL say
  the block takes a single attrset, not a list, e.g.:
  ```
  ["x"]: this is a single-nested block; pass one attrset { ... }, not a list
  ```
- Plain scalar/type mismatches keep their existing clear messages; only the
  collection-vs-object confusion gets the new, guidance-bearing text.

## Non-goals
- **Codegen emitting nested-block structure** (so the generated constructor
  carries the list/single nesting and users never guess). That is the larger,
  second half of beans-krwc and overlaps codegen / A5; it is tracked separately
  and is NOT in this change. This change makes the *error* actionable for all
  providers immediately, generated or hand-written.
- **Nix-side schema validation** (catching the mistake before apply, at eval
  time). Also valuable, also larger, also deferred; this change improves the
  executor-side error that everyone hits today.
- No IR shape change: this is purely the executor's value-coding error text. The
  IR contract is untouched.

## Impact
- Changed: `internal/tfcodec/tfcodec.go` (`sliceToValue` / `tupleToValue` detect a
  `map` and `mapToValue` detects a `[]interface{}`, returning the actionable
  error). Pure error-message change; no behavior change for valid input.
- Tests: `internal/tfcodec/tfcodec_test.go` (table-driven: list-nested block given
  a map -> guidance error naming the wrap fix; single-nested given a list ->
  guidance error; valid inputs still succeed unchanged).
- Beans: krwc (the error half).
