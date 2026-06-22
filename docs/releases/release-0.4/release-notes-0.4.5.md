# Release notes: v0.4.5 (remote state + locking)

The first slice of **M2 (team-ready)**: keep your state in a shared **S3 object**
instead of a local file, and a **lock** so two applies cannot corrupt it. This is
a patch release on the 0.4 line; the M2 milestone continues (drift detection and
multiple environments are still to come).

> Hands-on, runnable walkthrough: **[Tutorial: remote state on S3, with
> locking](../../TUTORIAL-REMOTE-STATE.md)**. It uses a real S3 bucket for state
> while keeping the resources offline (the fake providers), so you exercise the
> real remote-state path without creating billable infrastructure.

## Highlights

### Configure remote state in the flake

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
config. Absent `backend` keeps the local file store (unchanged). State is Nivis's
own format (no tfstate compatibility), and every write is server-side encrypted.

### State locking keeps concurrent applies safe

`apply` and `destroy` take an advisory lock (a small `<key>.lock` object created
atomically in S3) so two people or CI jobs cannot mutate the same state at once.
If a run is already holding it, the next one stops before doing anything:

```
error: state is locked by alice@ci-runner since 2026-06-22T10:31:04Z for "apply"; run `nivis force-unlock` to override
```

Read-only commands (`plan`, `refresh`, `output`, `state pull`) do not lock.

### force-unlock clears a stuck lock

If a run crashes while holding the lock, clear it with:

```sh
nivis force-unlock --attr nivis.remoteState
```

It confirms first (pass `--force` in CI). Only do this when you are sure no other
run is active.

## What shipped

- **B1 Remote state backend (S3 first)** — the IR `backend` block, an S3-backed
  `Store` (one object per state, server-side encrypted, AWS credential chain), and
  backend selection from the config. A hermetic in-repo fake S3 makes it testable.
- **B2 State locking** — an advisory lock on the S3 backend via an atomic
  conditional-put lock object, held across `apply`/`destroy`, with a
  `nivis force-unlock` escape hatch and "who holds it / since when" errors.

## Changelog

See the `[0.4.5]` section of [CHANGELOG.md](https://github.com/wearetechnative/nivis/blob/main/CHANGELOG.md).

## Try it

```sh
nix shell github:wearetechnative/nivis#nivis github:wearetechnative/nivis#fake-providers
export AWS_PROFILE=your-profile
# edit nix/example/remote-state.nix: set your bucket + region
nivis apply --attr nivis.remoteState     # state -> s3://<bucket>/nivis-tutorial/remote-state/app.json
nivis plan  --attr nivis.remoteState     # reads state back from S3 -> no changes
```
