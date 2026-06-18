# Proposal: colored-plan-output

## Why
The plan/apply/destroy output is hard to scan: every line is the same color, the
change type is a bare ASCII marker, and the phased nature of an apply (the thing
that makes Nivis Nivis) is invisible. Other IaC tools colorise by change type and
this is table stakes for a daily-driver (A3 of the "Road to v1" milestone,
beans-yqd3). There is already a tested `colorEnabled` helper (`cmd/nivis/splash.go`)
that respects `NO_COLOR` and non-TTY writers; this builds on it. This is pure DX:
no resource is planned, applied, or destroyed differently.

## What changes
- **Colorise by change type** in plan/apply/destroy: `+` create (green), `~`
  update (yellow), `-/+` replace (red+green), `-` destroy (red), `=` no-op (dim).
  A datasource **read** gets its own marker `r` in a distinct (dim/read) color so
  it is clearly a read, not a create.
- **Phase-grouped apply output.** Instead of a flat list, apply groups resources
  under the phase they resolved in (`Phase 1`, `Phase 2`, ...), making the
  fixpoint visible. This needs a small addition to the phase `Result`: a per-phase
  list of the ids applied in each phase, and each id's kind (resource vs
  datasource) so the read marker can be chosen.
- **Counts and summary** stay, restated cleanly (e.g. "Applied 9 resource(s)
  across 3 phase(s)").
- **Color is gated** by the existing `colorEnabled`: a non-TTY (piped) writer or
  `NO_COLOR` set produces plain ASCII with the same markers and the same text, so
  scripts and tests see stable output.
- **Testability:** plan/apply/destroy write through `cmd.OutOrStdout()` (cobra's
  capturable writer) via a small `output` helper, instead of `fmt.Printf` to the
  process stdout, so the rendering is unit-testable with color forced on and off.

## Decisions (settled with the maintainer)
- **Color + per-phase grouping** (full epic), not color-only.
- **Distinct read marker** (`r`) for datasource reads, not the same glyph as a
  create.

## Non-goals
- A `-detailed-exitcode` / drift-only mode, a TUI, or progress spinners.
- Changing the markers' meaning or the plan/apply/destroy behaviour.
- Per-attribute diffs (showing which attributes changed within a `~` update); the
  marker is at the resource level, as today.

## Impact
- Executor: `internal/phase` `Result` gains a per-phase grouping of applied ids
  and each applied id's kind (resource/datasource). Populated by the driver loop
  (which already tracks the phase and the node). No behaviour change.
- CLI: `cmd/nivis` gains a small `output` helper (a writer + the `colorEnabled`
  state + semantic painters keyed to the markers); plan/apply/destroy render
  through it and `cmd.OutOrStdout()`. `splash.go`'s ANSI constants/`colorEnabled`
  are reused (factored if needed).
- Tests: `cmd/nivis` unit tests render plan/apply/destroy to a buffer with color
  forced both ways (assert ANSI present on a color writer, absent on a plain one,
  markers and phase grouping correct); `internal/phase` test for the new
  per-phase grouping in `Result`.

Docs impact: modifications only; refresh the AWS S3 / EC2 tutorial apply-output
samples to the new phase-grouped form and note color is TTY-gated. No new document
or section: colored output is a presentation change to an already-documented flow,
not a new user-facing concept (per docs/DOCS-GATE.md).
