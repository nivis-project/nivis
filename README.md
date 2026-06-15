# nixform — seed kit

Starter artifacts for letting Claude Code ("cc") autonomously build the nixform
PoC. cc administers **beans** (milestones/epics) and uses **OpenSpec**
(proposals/tasks) for the work inside each epic.

## What nixform is
A Nix-native infra tool where Terraform/OpenTofu provider resources are
first-class Nix values, driven by a thin Go executor that speaks `tfprotov6` to
unmodified provider binaries. The thesis: provider-created values return into
Nix, which re-evaluates to produce dependent config — resolved across N phases
to a fixpoint.

## Files
- `CLAUDE.md` — cc's entry point. Read first. First action: `beans prime`.
- `DESIGN.md` — decision ledger (the invariants and why).
- `ROADMAP.md` — epics mapped to beans; the real critical-path ordering.
- `docs/IR-CONTRACT.md` — the frozen cross-epic IR (the linchpin).
- `docs/TESTING.md` — test layers + the headline two-provider e2e (milestone exit).
- `scripts/bootstrap-beans.sh` — seeds the milestone + epics in beans.
- `openspec/` — conventions + a fully worked example change (`define-ir-contract`)
  to mirror.

## Getting started (cc)
1. `beans init` (if needed) then `bash scripts/bootstrap-beans.sh`.
2. `beans prime`, then read CLAUDE.md → DESIGN.md → ROADMAP.md → docs/IR-CONTRACT.md.
3. Implement `openspec/changes/define-ir-contract` first (E1.5, the linchpin).
4. Follow the critical path in ROADMAP.md. Prove the round trip before codegen.

Verify exact `beans` / `openspec` flag names against their CLIs — both tools are
evolving; adjust the bootstrap script and conventions if a flag differs.
