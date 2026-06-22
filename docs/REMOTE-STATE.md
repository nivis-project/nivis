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

## Locking (not yet)

This backend does **not** yet serialize concurrent applies. Each operation is an
atomic read-modify-write of the state object, but two people (or two CI jobs)
applying the same stack at the same time can still race. **State locking is the
next M2 epic (B2)** and will add a distributed lock with a `force-unlock` escape
hatch. Until then, coordinate so only one apply runs at a time.

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
