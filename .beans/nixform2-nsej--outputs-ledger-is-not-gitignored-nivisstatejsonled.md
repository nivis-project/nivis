---
# nixform2-nsej
title: 'Outputs ledger is not gitignored: nivis.state.json.ledger escapes the rules'
status: completed
type: bug
priority: high
tags:
    - discovered
created_at: 2026-09-24T14:51:03Z
updated_at: 2026-09-29T19:22:32Z
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

## Investigated 2026-09-24: bigger than first filed

Four corrections and additions after reading the code.

### The ledger is where secrets live BY DESIGN, not merely "may hold" them

`docs/IR-CONTRACT.md`, "Sensitive values across the boundary", routes every
sensitive provider output away from the world-readable Nix store and into the
ledger as the one plaintext copy:

> Sensitive outputs must not be written into the IR JSON emitted by `nix eval`
> (that output and the Nix store are world-readable) ... Live only in the
> executor's outputs ledger, which is written with restricted permissions (0600)
> ... This is a hard requirement; getting it wrong leaks secrets into the store.

The 0600 half is implemented correctly (`internal/ledger/ledger.go:62-72`, chmod
enforced so umask cannot widen it). The git half is wide open.

### No fixed-name rule can cover it: the name is derived from --state

```
cmd/nivis/main.go:87    --state defaults to ./nivis.state.json
cmd/nivis/main.go:400   LedgerPath: statePath + ".ledger"
```

So `--state prod.json` writes `prod.json.ledger`. The original fix note in this
bean ("add the pattern nivis actually writes") is wrong: there is no single such
pattern. Only a `*.ledger` suffix rule covers every case.

### The local STATE file is un-ignored too, not just the ledger

```
$ git check-ignore -v nivis.state.json nivis.state.json.ledger prod.json.ledger
(no output: none of them are ignored)
```

All three rules in the runtime-artifacts section are dead after the rename:

| rule                       | covers                  |
|----------------------------|-------------------------|
| `*.ledger.json`            | nothing (extension is backwards) |
| `terrae-nivis.state.json*` | nothing (stale rename)  |
| `/.terrae-nivis/`          | nothing (stale rename)  |

The section is decorative. State carries secrets too: the motivating example in
`nixform2-rlbz` was a GitHub token in Amplify config sitting in state.

### The half that affects USERS, missing from the original filing

nivis writes state and ledger into the **user's** working directory by default, so
this repo's `.gitignore` does nothing for them. No documentation tells a user to
ignore either file; `docs/REMOTE-STATE.md:266` is the only mention of `.ledger`
anywhere in docs, and it is describing what `state migrate` removes. Every nivis
user running against a real provider therefore has a secret-bearing file in their
repository root that nothing warns them about. This is arguably more serious than
our own `.gitignore`.

## Fix, in three independent pieces

1. `.gitignore`: add `*.ledger` and `nivis.state.json*`, and DELETE the three dead
   rules rather than leaving them to imply coverage they do not provide.
2. Docs: say what nivis writes into the working directory and that neither file
   may be committed, where `nivis apply` first appears in the tutorials, which is
   when the files show up.
3. Optional, bigger call: write them somewhere less committable by default, or
   print a one-time note on first write. This changes where people's state lives,
   so it wants its own decision.

Raised to high: piece 1 is one line and the exposure is live today.


### Correction: the three rules fail in two different ways

An earlier note here called all three rules equally dead. True, but only two of
them protect something that still exists, and only those need replacing.

Two renames happened; the gitignore only ever tracked the first:

```
nixform       -> terrae nivis   (2a553df)   <- /.terrae-nivis/ added here
terrae nivis  -> nivis          (cb957ea)   <- never updated
```

| rule                       | what it guarded        | action   |
|----------------------------|------------------------|----------|
| `*.ledger.json`            | the outputs ledger, which still exists | REPLACE with `*.ledger` |
| `terrae-nivis.state.json*` | the local state file, which still exists | REPLACE with `nivis.state.json*` |
| `/.terrae-nivis/`          | a repo-local dir nothing creates any more | DELETE |

`/.terrae-nivis/` needs deleting, not renaming. Nothing writes a repo-local
dot-directory today. Every `MkdirAll` in the tree was checked: the provider cache
(the closest thing that directory was for) lives at `<user cache>/nivis/providers`
(`internal/registry/registry.go:26-28`), outside any repo; codegen output
(`cmd/nivis/gen.go:63`) and the tutorial scaffold are caller-chosen and meant to be
committed. Adding a `.nivis/` in its place would be cargo-culting a directory that
does not exist.

The only `terrae-nivis` left in Go is the `TERRAE_NIVIS_FAKE_COUNTER` env var in
the fake providers, which is a test seed and not a path. Harmless, but it is
another artifact of the same incomplete rename if anyone is tidying.

### A custom --state path is not covered by a name-anchored rule

`internal/state/state.go:68` creates the directory of whatever `--state` names, so
`--state build/state.json` silently creates `build/` and drops a secret-bearing
pair inside it.

- `*.ledger` is a suffix rule, so it catches the ledger wherever it lands.
- `nivis.state.json*` is anchored to the DEFAULT name, so it does not catch
  `build/state.json`.

Closing that fully means either a broader `*.state.json*`, or deciding that a
custom `--state` path is the user's responsibility to ignore. That is a judgement
call for whoever picks this up, not an oversight. It is another reason the
documentation piece matters: a rule can only cover the default, while docs can
tell a user what to ignore whatever path they choose.

## Summary of Changes

OpenSpec change: `warn-and-ignore-runtime-secrets`.

Three pieces, because a `.gitignore` alone only ever protects this repository.

### Our own ignore rules

`*.ledger` and `nivis.state.json*` added; `*.ledger.json`,
`terrae-nivis.state.json*` and `/.terrae-nivis/` deleted, all three having
matched nothing since the rename. Verified rather than assumed:

```
$ git check-ignore -v nivis.state.json nivis.state.json.ledger prod.json.ledger build/x.json.ledger
.gitignore:40:nivis.state.json*   nivis.state.json
.gitignore:40:nivis.state.json*   nivis.state.json.ledger
.gitignore:39:*.ledger            prod.json.ledger
.gitignore:39:*.ledger            build/x.json.ledger
```

`/.terrae-nivis/` was deleted rather than renamed: nothing writes a repo-local
dot-directory any more, and the provider cache lives outside any repository.

### The notice

A run that CREATES the state document or the ledger now says so once, naming the
actual paths and that they must not be committed. This is the only piece that
reaches a user who reads neither our ignore rules nor our docs, since the files
land in THEIR working directory.

Creation is observed by comparing existence before and after the run rather than
by reporting from the write sites. Same behaviour, no callback threaded through
two packages, and it covers a case the task list did not anticipate: with an S3
backend there is no local state file, but the ledger is still written to the
working directory, because `LedgerPath` is derived from the state path regardless
of backend.

Eight tests, including that the notice never echoes file contents (seeded with a
fake token and asserted absent) and that several writes in one run still produce
one notice.

### Documentation

The AWS S3 tutorial where state first appears, and a new section in
`docs/REMOTE-STATE.md`. Both say that a file already committed needs removing
from the repository and its secrets rotating: adding it to `.gitignore` leaves it
in history.

### Considered and deliberately left out

Moving the files somewhere less committable. It is the only change that removes
the exposure rather than reducing it, and it changes where everyone's state
lives, so it wants its own decision rather than riding along with a gitignore
edit. This is not an oversight.

Also left out: a broad `*.state.json*` rule. A rule can only cover the default
path, while the notice names whatever `--state` actually produced, which covers
the custom case with something that cannot be wrong.
