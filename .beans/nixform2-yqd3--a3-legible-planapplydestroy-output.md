---
# nixform2-yqd3
title: 'A3: Legible plan/apply/destroy output'
status: completed
type: epic
priority: normal
tags:
    - roadmap
created_at: 2026-06-16T13:37:59Z
updated_at: 2026-06-18T07:53:56Z
parent: nixform2-zdj0
---

Make the plan readable: colorise by change type and make the phased nature visible. Pure DX, no behaviour change.

ROADMAP Phase A3. The user explicitly asked for colored output of plan/apply/destroy. GROUND TRUTH: a branded splash exists (cmd/nivis/splash.go) but the plan/apply diff is not colorised by change type.

## Scope
- Colorise: + create (green), ~ update (yellow), -/+ replace (red+green), - destroy (red), = no-op (dim).
- Summarise counts; show which resources resolved in which phase.
- Respect NO_COLOR and non-TTY (no escape codes when piped).


---
## Summary of Changes
DONE via OpenSpec change colored-plan-output (archived 2026-06-18-colored-plan-output). Pure DX, no behaviour change:

- COLOR: plan/apply/destroy colorise by change type: + create (green), ~ update (yellow), -/+ replace (red+green), - destroy (red), = no-op (dim), and a datasource READ gets a distinct dim `r` marker (not a create glyph). A new cmd/nivis/output.go helper (writer + colorEnabled state + semantic painters) reuses splash.go's colorEnabled (NO_COLOR + TTY gate) and ANSI consts; added green/yellow/red. Output goes through cmd.OutOrStdout() (capturable), not fmt.Printf to process stdout.
- PHASE GROUPING: apply groups resources under "Phase 1 / Phase 2 / ..." headings (ice-blue), making the fixpoint visible. The phase Result gained Phases [][]AppliedNode (id + IsData kind), populated by the driver loop; flat Applied + AppliedPhases kept for back-compat.
- GATING: a non-TTY (piped) writer or NO_COLOR => NO ANSI, same markers/text/grouping (stable for scripts/tests).

DECISIONS (with maintainer): color + per-phase grouping (full epic, not color-only); distinct `r` read marker for datasources (not the create glyph).

TESTS: cmd/nivis output_test.go (plain-no-ANSI with all six markers, forced-color emits each ANSI, NO_COLOR disables, phase heading); internal/phase per-phase grouping (3 groups, concatenation == Applied, datasource marked read). Visually verified the rendering. Full gate green: gofmt, go build, go test, check-docs-ssot (incl docs-coverage + mdbook).

DOCS (modifications only): refreshed the AWS S3 + EC2 tutorial apply samples to the phase-grouped form (markers + the per-phase layering; EC2's 4-phase grouping derived from the dependency graph) and noted color is TTY-gated.

NON-GOALS (deferred): detailed-exitcode/drift-only mode, TUI/spinners, per-attribute diffs within an update.
