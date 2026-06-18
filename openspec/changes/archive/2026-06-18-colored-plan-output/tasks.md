# Tasks: colored-plan-output

## 1. Executor: per-phase grouping in the result
- [x] 1.1 `internal/phase`: `Result` gains `Phases [][]Applied` (or similar): the
      ids applied in each phase, in phase order, each carrying its id + kind
      (resource | datasource). Keep `Applied` (flat) and `AppliedPhases` (count)
      for back-compat.
- [x] 1.2 The driver loop records, per phase, the ids it applied and whether each
      was a read (node.Resource.IsData) or an apply; `finish` populates `Phases`.
- [x] 1.3 Unit test: a 3-phase chain yields 3 phase groups whose concatenation
      equals `Applied`; a datasource id is marked as a read.

## 2. CLI: the output helper
- [x] 2.1 `cmd/nivis`: a small `output` type wrapping an `io.Writer` + the
      `colorEnabled(w)` state, with semantic painters: `create`/`update`/
      `replace`/`destroy`/`noop`/`read`, each emitting `<marker> <text>` and an
      ANSI color when enabled, plain otherwise. Reuse `splash.go`'s ANSI consts +
      `colorEnabled`; add green/yellow/red as needed.
- [x] 2.2 Markers: `+` green, `~` yellow, `-/+` red+green, `-` red, `=` dim,
      `r` dim/read. Plain-mode keeps the same markers and text.

## 3. CLI: render plan/apply/destroy through it
- [x] 3.1 `plan`: render each item through `output` keyed to its `Op`; keep the
      "N change(s) ... Run nivis apply" / "No changes" summary, colorised.
- [x] 3.2 `apply`: group by `res.Phases` (`Phase 1` ... headings), each node with
      its marker (read marker for datasources); summary "Applied N across M
      phase(s)". Write via `cmd.OutOrStdout()`.
- [x] 3.3 `destroy`: render the destroyed list with the `-` destroy marker/color
      via `cmd.OutOrStdout()`.

## 4. Tests
- [x] 4.1 `cmd/nivis`: render plan to a buffer (a *bytes.Buffer is non-TTY, so
      colorEnabled=false): assert markers + text + summary, NO ANSI.
- [x] 4.2 A test that forces color on (a small seam to override colorEnabled, or
      render a known PlanItem set) and asserts ANSI codes appear for each type.
- [x] 4.3 apply rendering: a result with `Phases` set prints phase headings in
      order and the read marker for a datasource id.
- [x] 4.4 NO_COLOR test: with NO_COLOR set, even a forced path emits no ANSI.

## 5. Docs (docs-coverage gate: modifications only)
- [x] 5.1 Refresh the AWS S3 + EC2 tutorial apply-output samples to the new
      phase-grouped form; note color is TTY-gated. No new doc.

## 6. Gate
- [x] 6.1 `gofmt`, `go build ./...`, `go test ./...` green.
- [x] 6.2 `bash tests/check-docs-ssot.sh` green (docs touched).
- [x] 6.3 `openspec validate colored-plan-output --strict`; archive; close
      beans-yqd3.
