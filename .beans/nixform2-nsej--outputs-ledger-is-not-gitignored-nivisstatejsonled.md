---
# nixform2-nsej
title: 'Outputs ledger is not gitignored: nivis.state.json.ledger escapes the rules'
status: todo
type: bug
tags:
    - discovered
created_at: 2026-09-24T14:51:03Z
updated_at: 2026-09-24T14:51:03Z
parent: nixform2-kovh
---

`.gitignore` says the outputs ledger MUST NOT be committed, because it may hold
sensitive provider outputs (see the comment above the rule, and
docs/IR-CONTRACT.md "Sensitive values across the boundary").

The rules do not match the file nivis actually writes:

```
.gitignore:  *.ledger.json
.gitignore:  terrae-nivis.state.json*
written:     nivis.state.json.ledger
```

Neither pattern matches. `*.ledger.json` expects the extension the other way
round, and `terrae-nivis.state.json*` is from before the rename to nivis. So a
real run leaves an un-ignored ledger in the working directory.

## How it surfaced

Found while shipping `configurable-s3-backend-encryption` (bean nixform2-rlbz).
A manual test run against a real S3 bucket wrote `nivis.state.json.ledger`, and
it showed up as a normal untracked file that `git add -A` staged. It was caught
before the commit. The content there was only fake-provider outputs, but against
a real provider it is exactly what the rule exists to keep out of git.

## Fix

Add the pattern nivis actually writes, e.g. `*.ledger` (or
`nivis.state.json.ledger` plus `*.state.json.ledger`), and check whether the
stale `terrae-nivis.*` rules still earn their place after the rename. Worth
checking `/.terrae-nivis/` in the same pass.

Consider also whether nivis should write the ledger somewhere less likely to be
committed than the working directory root.
