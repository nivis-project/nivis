# Tasks: s3-state-locking

## 1. The Locker seam
- [x] 1.1 `internal/state/lock.go`: a `Locker` interface
      (`Lock(LockInfo) (string, error)`, `Unlock(id string) error`,
      `ForceUnlock() error`) and a `LockInfo` struct (who/host/pid/time/operation
      + a generated lock id). `NewLockInfo(operation)` fills it from the
      environment. `Store` stays unchanged; backends MAY implement `Locker`.

## 2. S3 conditional-put lock
- [x] 2.1 `internal/state/s3.go`: `s3Store` implements `Locker`. Lock object at
      `<key>.lock`. `Lock` = PutObject(IfNoneMatch:"*") of the LockInfo JSON;
      `PreconditionFailed`/412 => read the existing lock object and return an error
      naming the holder (user@host, since, operation) and pointing at
      `nivis force-unlock`.
- [x] 2.2 `Unlock(id)` = GetObject the lock, refuse if its id != id (don't drop
      someone else's lock), else DeleteObject. `ForceUnlock` = DeleteObject
      unconditionally (no-op if absent).

## 3. Fake S3 conditional put
- [x] 3.1 `internal/fakes3`: honor `If-None-Match: *` on PUT — return 412
      (PreconditionFailed, S3 XML) when the object already exists; otherwise
      create. (Existing unconditional PUTs for state are unaffected.)

## 4. CLI: lock around mutating runs + force-unlock
- [x] 4.1 `cmd/nivis`: in `apply` and `destroy`, if the store is a `Locker`,
      acquire the lock (NewLockInfo("apply"/"destroy")) before the run and release
      it deferred after; print "Acquired/Released state lock". Read-only commands
      (plan/refresh/output/state pull) do NOT lock.
- [x] 4.2 `nivis force-unlock`: clears a stuck lock via `ForceUnlock`, with a
      confirm prompt (and `--force`/`--yes` for non-interactive), naming what was
      unlocked. Errors clearly if the backend is not a Locker (e.g. local store).

## 5. Tests
- [x] 5.1 `internal/state`: lock unit tests against the fake S3 — Lock succeeds and
      creates `<key>.lock`; a second Lock fails with the holder's user/since in the
      error; Unlock(id) releases; Unlock with a wrong id is refused; ForceUnlock
      clears a held lock; Lock after release succeeds.
- [x] 5.2 `cmd/nivis`: a `force-unlock` test (clears a lock; refuses/〈errors〉 on a
      non-Locker store).
- [x] 5.3 e2e (`tests/e2e`): with an s3 backend on the fake S3, hold the lock, then
      assert a second acquire is rejected with the holder info; release; assert it
      now succeeds. (Drives the state Locker directly; hermetic.)

## 6. Docs
- [x] 6.1 `docs/REMOTE-STATE.md`: replace the "no locking yet (B2)" caveat with the
      locking behaviour — automatic on apply/destroy, the holder error, and
      `nivis force-unlock`. No em dashes.

## 7. Changelog
- [x] 7.1 `CHANGELOG.md` `[Unreleased]` `### Added`: state locking (matches the
      proposal's `Changelog:` line).

## 8. Gate
- [x] 8.1 `gofmt`, `go build ./...`, `go test ./...` green.
- [x] 8.2 `bash tests/run-nix-tests.sh` + `bash tests/check-docs-ssot.sh` green.
- [x] 8.3 `openspec validate s3-state-locking --strict`; archive; complete
      beans-0oqk (B2 delivered) and note B3 (drift) is next; M2 stays in-progress.
