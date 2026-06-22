# Tutorial: remote state on S3, with locking (hands-on)

This walks you through the M2 team-ready features added in this release: keeping
your state in a shared **S3 object** instead of a local file, and the **lock** that
stops two applies from corrupting it.

To keep it focused on *state* (not cloud resources), the resources in this tutorial
are the offline **fake providers** — but the **state** is stored in a real S3
bucket. So you exercise the real remote-state path (and real locking, a real lock
object, a real `force-unlock`) without creating any billable infrastructure.

**Prerequisites:** Nix on your `PATH`, an S3 bucket you own, and AWS credentials in
your environment (the AWS default chain). Set `AWS_PROFILE` (or
`AWS_ACCESS_KEY_ID`/…). Credentials are never in the config: only the bucket/key/
region are.

## Setup

Enter a shell with `nivis` and the fake providers, and point AWS at your profile:

```sh
nix shell github:wearetechnative/nivis#nivis github:wearetechnative/nivis#fake-providers
export AWS_PROFILE=your-profile
```

The config for this tutorial ships as the flake attribute `nivis.remoteState`
(`nix/example/remote-state.nix`). Open it and set the **bucket** and **region** to
one you own:

```nix
backend = {
  type = "s3";
  bucket = "your-state-bucket";              # <- yours
  key = "nivis-tutorial/remote-state/app.json";
  region = "eu-west-1";                      # <- your bucket's region
};
```

That `backend` block is the whole feature: it tells Nivis to store state in
`s3://your-state-bucket/nivis-tutorial/remote-state/app.json` instead of a local
file.

<!-- release-note: Configure remote state in the flake -->
State lives where your **config** says, not where a flag says. Declare a `backend`
on `toIR` and `nivis` reads and writes state there:

```nix
toIR {
  backend = { type = "s3"; bucket = "your-state-bucket"; key = "prod/app.json"; region = "eu-west-1"; };
  providers = { /* ... */ };
  resources = [ /* ... */ ];
  inherit ledger;
}
```

Credentials come from the AWS chain (`AWS_PROFILE`/keys); only the location is in
config. Absent `backend` keeps the local file store (unchanged).
<!-- /release-note -->

## 1. Apply: state goes to S3

```sh
AWS_PROFILE=$AWS_PROFILE nivis apply --attr nivis.remoteState
```

```
Acquired state lock.
Applied 2 resource(s) across 2 phase(s):

Phase 1
  + alpha.alpha_token.app
Phase 2
  + beta.beta_record.app
Released state lock.
```

Notice the **Acquired/Released state lock** lines: `apply` took the lock for the
duration of the run. Now look in your bucket — the state object is there:

```sh
aws s3 ls s3://your-state-bucket/nivis-tutorial/remote-state/
# app.json
```

There is **no local `nivis.state.json`**: the state of record is the S3 object.

## 2. Plan reads state back from S3

```sh
AWS_PROFILE=$AWS_PROFILE nivis plan --attr nivis.remoteState
```

```
  = alpha.alpha_token.app (alpha_token)
  = beta.beta_record.app (beta_record)

No changes. 2 resource(s) up to date.
```

`plan` read the state straight from S3 and saw nothing to do. (Read-only commands
like `plan` do not take the lock.)

## 3. Locking: concurrent applies are kept apart

<!-- release-note: State locking keeps concurrent applies safe -->
`apply` and `destroy` take an advisory lock (a small `<key>.lock` object created
atomically in S3) so two people or CI jobs cannot mutate the same state at once.
If a run is already holding it, the next one stops before doing anything:

```
error: state is locked by alice@ci-runner since 2026-06-22T10:31:04Z for "apply"; run `nivis force-unlock` to override
```
<!-- /release-note -->

You can see this for yourself: in one terminal, hold the lock by pausing an apply
(or simulate a crashed run), and in another run `nivis apply` — it refuses with the
holder's name and time.

If a run crashes while holding the lock, the lock object is left behind and the
next run is blocked. Clear it:

<!-- release-note: force-unlock clears a stuck lock -->
```sh
AWS_PROFILE=$AWS_PROFILE nivis force-unlock --attr nivis.remoteState
```

It confirms first (pass `--force` in CI). Only do this when you are sure no other
run is active.
<!-- /release-note -->

## 4. Read outputs and clean up

```sh
AWS_PROFILE=$AWS_PROFILE nivis output --attr nivis.remoteState
AWS_PROFILE=$AWS_PROFILE nivis destroy --attr nivis.remoteState
```

`destroy` also takes the lock. When you are done, remove the state object (and any
leftover `.lock`) from your bucket:

```sh
aws s3 rm s3://your-state-bucket/nivis-tutorial/remote-state/app.json
aws s3 rm s3://your-state-bucket/nivis-tutorial/remote-state/app.json.lock 2>/dev/null || true
```

## Notes

- State is **Nivis's own format**: it is not a Terraform/OpenTofu `tfstate` file
  and the two are not interchangeable.
- Every write to the state object requests **server-side encryption** (AES256).
- To migrate an existing local state into S3, use `nivis state pull --out
  state.json` with the local config, then switch the config to the s3 backend and
  `nivis state push --in state.json --force`.

See [Remote state (the S3 backend)](REMOTE-STATE.md) for the full reference.
