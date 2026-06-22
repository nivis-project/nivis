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

## Migrating local state to S3

`nivis state pull` / `nivis state push` move the whole state document, so you can
seed an S3 backend from an existing local state:

```sh
# with the LOCAL config (no backend), export the document:
nivis state pull --out state.json

# switch the config to the s3 backend, then import it:
nivis state push --in state.json --force
```

`pull`/`push` operate on the document through whichever backend the config
selects, so the same commands work in both directions.
