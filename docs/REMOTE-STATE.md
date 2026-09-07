# Remote state (the S3 backend)

By default Nivis keeps state in a local JSON file (`--state`, default
`./nivis.state.json`). For a team or CI, declare a **remote backend** so state
lives in a shared store instead. The first backend is **S3**.

State stays Nivis's own format. There is **no tfstate compatibility** (a
deliberate design choice): Nivis state is not a Terraform/OpenTofu state file and
the two are not interchangeable.

## Configure it in the flake

The backend is part of your configuration, not a flag or an env var. Declare it
with `backend` on your top-level config (it flows through the IR to the executor):

```nix
{ nivis }:
ledger:
nivis.toIR {
  backend = {
    type = "s3";
    bucket = "my-company-nivis-state";
    key = "prod/app.json";          # the object key for THIS stack's state
    region = "eu-west-1";
  };
  providers = { /* ... */ };
  resources = [ /* ... */ ];
  inherit ledger;
}
```

Then `nivis plan` / `nivis apply` read and write state in
`s3://my-company-nivis-state/prod/app.json` instead of a local file. The whole
state document is stored as that one object.

### Keys

- `type` (required): `"s3"`.
- `bucket`, `key`, `region` (required for s3): where the state object lives.
- `endpoint` (optional): override the S3 endpoint (for an S3-compatible store or a
  test server). Unset in production, where the AWS SDK resolves the real endpoint.

The backend is **static**: its values must be plain (no references to resource
outputs), because the executor has to know where state lives before it evaluates
anything.

## Credentials

Credentials are **never** in the config. The S3 backend uses the **AWS default
credential chain** (the same chain the AWS CLI/SDK use): environment variables
(`AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` / `AWS_PROFILE`), the shared
credentials/config files, or an instance/role profile. Set `AWS_PROFILE` (or the
keys) in your shell or CI; the config carries only the location.

## Encryption

Every write requests **server-side encryption** (`AES256`) on the state object.
Enable bucket policies/versioning on your side as you would for any state bucket.

## Locking

`nivis apply` and `nivis destroy` take an **advisory lock** on the state before
they run, so two people (or two CI jobs) cannot mutate the same stack at the same
time and corrupt it. The lock is a small sibling object next to your state object
(`<key>.lock`), created atomically with an S3 conditional write (no DynamoDB or
other service is needed). It is released automatically when the run finishes,
including when it fails.

Read-only commands (`plan`, `refresh`, `output`, `state pull`) do not lock.

If a run is already holding the lock, the next `apply`/`destroy` stops before
doing anything and tells you who holds it and since when:

```
error: state is locked by alice@ci-runner since 2026-06-22T10:31:04Z for "apply"; run `nivis force-unlock` to override
```

### force-unlock

If a run crashes (or is killed) while holding the lock, the lock object is left
behind and the next run is blocked. Clear it with:

```sh
nivis force-unlock
```

It confirms first in an interactive shell; pass `--force` (or `--yes`) to skip the
prompt in CI. **Only force-unlock when you are sure no other run is active**, or you
risk two concurrent applies. The local file store does not use this lock (it is
single-machine and has its own per-operation file lock), so `force-unlock` there
reports there is nothing to clear.

## Moving state between backends

`nivis state migrate` moves the whole state document between your local state file
and the backend your configuration declares, in the direction you choose:

```sh
nivis state migrate --to-remote     # local state file  ->  the declared backend
nivis state migrate --from-remote   # the declared backend  ->  local state file
```

It reports both sides before it does anything, and on success it says how many
resources moved:

```
Migrating the state document
  from the local state file ./nivis.state.json
  to   s3://my-company-nivis-state/prod/app.json
Moved 7 resource(s) to s3://my-company-nivis-state/prod/app.json; removed the source document at ./nivis.state.json.
```

You must pass exactly one direction — Nivis will not guess which way your state of
record should move. If your configuration declares no `backend`, there is nothing
to migrate to or from and the command says so.

### What it does, in order

1. Takes the state **lock** on both sides (for backends that support locking), so a
   migration cannot race an `apply`.
2. Copies the document to the destination.
3. **Verifies** the destination by reading it back.
4. Only then **removes the source document** — the local state file (along with its
   derived `.ledger` and `.lock` siblings), or the remote state object.

If anything fails before step 4, the source document is untouched and the error
says so, so nothing is lost and you can re-run the command. If a run is
interrupted *between* steps 3 and 4, the destination already holds the document:
re-running finishes the job (it reports "an interrupted migration") instead of
complaining that the destination is occupied.

Because the source is removed, you never end up with a stale local state file
sitting next to a live remote backend — the single most confusing state a project
can be in. If you want a copy first, take one with `nivis state pull --out
state.json` (below).

### When it refuses

A migration will not silently destroy state you did not mean to overwrite:

| Destination                                   | Result                       |
|-----------------------------------------------|------------------------------|
| No state document, or an empty one            | proceeds                     |
| Exactly the document being migrated           | proceeds (finishes the move) |
| Different resources                           | **refused** — needs `--force` |
| Content that is not a Nivis state document    | **refused** — needs `--force` |

A refusal reports the resource count on both sides and changes nothing:

```
error: the destination already holds 3 resource(s) and the source holds 7;
refusing to overwrite state that is not a copy of the source (pass --force to overwrite it anyway)
```

Pass `--force` (or `--yes`) only when you are sure the destination's state is
disposable. Enable **bucket versioning** on your state bucket: it is the backstop
both for a forced overwrite and for the source-removal step.

### The lower-level pair: pull and push

`nivis state pull` / `state push` still move the document as bytes, and remain the
tool for anything `migrate` does not cover — taking a backup, editing a document,
or seeding a backend from a file:

```sh
nivis state pull --out state.json          # export through the selected backend
nivis state push --in state.json --force   # replace through the selected backend
```

Both operate on whichever backend the configuration currently selects, so
`migrate` is the one-step version of the old export/edit-config/import dance.

## Bootstrapping a self-managed state bucket

A common setup has a configuration create the very bucket it declares as its own
backend. That is a chicken-and-egg problem: on the first run the bucket does not
exist, so state cannot live there yet. Bootstrap it in three commands:

```sh
# 1. Apply with local state, creating the bucket (the declared backend is ignored
#    for this run only; the configuration is not modified).
nivis apply --backend=local

# 2. Move the state document into the bucket that now exists.
nivis state migrate --to-remote

# 3. From here on, ordinary runs use the declared backend.
nivis apply       # reports no changes
```

`--backend=local` is the escape hatch: it makes a single run use the local state
file at `--state` even though the configuration declares a remote backend. When it
overrides a declared backend, the run says so, so it is never ambiguous which state
a run operated on:

```
Using local state at ./nivis.state.json (--backend=local overrides the s3 backend this configuration declares).
```

It applies to every state-using command (`plan`, `apply`, `destroy`, `refresh`, and
the `state` subcommands) and affects only the run it is given on. `local` is its
only accepted value.

## A missing bucket is an error, not empty state

Nivis distinguishes two things that look similar and are not:

- **A missing state object** — the bucket exists, but no state has been written
  yet. This is a fresh stack: reads return an empty state document and the first
  write creates the object.
- **A missing state bucket** — the state *location* itself is unreachable. This is
  an error on every operation:

```
error: state bucket "acme-nivis-state" does not exist in eu-west-1: the state LOCATION is
missing, not just the state document, so this is not treated as an empty state (that would
re-create resources you already own).
  If this configuration creates that bucket, bootstrap it with:
      nivis apply --backend=local
      nivis state migrate --to-remote
  Otherwise correct backend.bucket / backend.region in your configuration.
```

The reason for the distinction is blunt: "empty state" and "I cannot see your
state" are opposite instructions to `apply`. The first means create; the second,
if mistaken for the first, means re-create infrastructure you already own. So a
missing bucket never reads as an empty stack — it stops the run and tells you
which of the two situations you are in. A failure that is neither (a permission
denial, say) keeps reporting its own cause.
