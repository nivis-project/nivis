# Tasks: state-ergonomics

## 1. Store: Snapshot/Restore + lock timeout
- [x] 1.1 `internal/state`: add `Snapshot() ([]byte, error)` (marshal the whole
      document under lock) and `Restore([]byte) error` (parse+validate as a
      document, then atomic write under lock) to the `Store` interface and
      `fileStore`. Restore rejects malformed input before touching the file.
- [x] 1.2 `withLock`: acquire with a timeout (LOCK_EX|LOCK_NB in a short retry
      loop, or flock with deadline); on timeout return an actionable error naming
      the lock file ("state appears locked by another process: <lock path>").
      Add a tunable (a field/arg) with a sane default (~5s).

## 2. Store tests
- [x] 2.1 Snapshot -> Restore on a fresh store reproduces the resource set.
- [x] 2.2 Restore replaces (a store with X, restore a snapshot with only Y -> Y,
      no X).
- [x] 2.3 Restore of malformed bytes errors and leaves existing state unchanged.
- [x] 2.4 A held lock (open the lock file + flock it in the test) makes an op
      return the actionable timeout error instead of hanging.

## 3. CLI: pull/push + polish
- [x] 3.1 `state pull`: write `store.Snapshot()` to stdout or `--out <file>`.
- [x] 3.2 `state push`: read stdin or `--in <file>`; report incoming vs current
      counts; prompt for confirmation; `--force`/`--yes` skips; **require
      --force when stdin is not a TTY** (refuse with a clear message otherwise);
      then `store.Restore`.
- [x] 3.3 Polish: `rm` of a missing id reports "not in state" (not silent);
      `show` of a missing id already errors (keep, ensure clear); `list` prints a
      note when empty.

## 4. CLI tests
- [x] 4.1 push from a non-TTY without `--force` is refused, state unchanged.
- [x] 4.2 push with `--force` (from a buffer) restores the document.
- [x] 4.3 rm of a missing id reports it.

## 5. Docs (docs-coverage gate: new section)
- [x] 5.1 `docs/INSTALL.md` (or a short state section): "Move state around
      (pull/push)" + the lock-error note. No em dashes.

## 6. Gate
- [x] 6.1 `gofmt`, `go build ./...`, `go test ./...` green.
- [x] 6.2 `bash tests/check-docs-ssot.sh` green (docs touched).
- [x] 6.3 `openspec validate state-ergonomics --strict`; archive; close beans-z8e1.
