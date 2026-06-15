#!/usr/bin/env bash
# Bootstrap the beans tracker for terrae nivis: one milestone + the epics under it,
# wired with the real critical-path dependency edges (see ROADMAP.md).
#
# Verified against beans 1.4.x CLI (2026-06): create uses --parent for hierarchy
# and --blocked-by/--blocking for dependencies; there is NO --add-child. Valid
# priorities are critical|high|normal|low|deferred (NOT "medium"). IDs are
# auto-generated, so we capture each from --json and wire relationships by id.
#
# NOT idempotent: re-running creates duplicates. Run once on a fresh .beans/.
# Claude Code ("cc") owns beans administration; this just seeds the structure.
#
# Prereqs: `beans` on PATH. `beans init` is run automatically if needed.

set -euo pipefail

command -v beans >/dev/null || { echo "beans not found on PATH"; exit 1; }
command -v python3 >/dev/null || { echo "python3 needed to parse JSON ids"; exit 1; }
[ -d .beans ] || beans init

# Create a bean and echo its generated id. Args: title, then extra flags.
mk() {
  local title="$1"; shift
  beans create "$title" --json "$@" \
    | python3 -c 'import sys,json; print(json.load(sys.stdin)["bean"]["id"])'
}

echo "Seeding terrae nivis milestone + epics..."

# --- Milestone -------------------------------------------------------------
MILESTONE=$(mk "terrae nivis PoC / alpha base" \
  --type milestone --priority high --status todo --tag poc \
  --body "Exit criterion: headline two-provider e2e passes (unknowns on both \
sides, >=3 phases, Nix-side consumer reads both providers). See ROADMAP.md and \
docs/TESTING.md. cc administers this milestone and its epics.")
echo "  milestone: $MILESTONE"

# --- Epics (children of the milestone via --parent) ------------------------
# The critical path (ROADMAP.md) is NOT numeric order. We encode the real
# ordering as --blocked-by edges so `beans list --no-blocked` surfaces the next
# unblocked epic. cc should still consult ROADMAP.md for rationale.

E1=$(mk "E1 Nix library core (terrae-nivis-lib)" \
  --type epic --priority high --status todo --parent "$MILESTONE" --tag critical-path \
  --body "mkResource, reference system, meta-args, module system, IR serializer, \
flake interface. Tasks tracked as OpenSpec changes. See ROADMAP.md Epic 1. \
OpenSpec changes: (record change-ids here as created).")
echo "  E1:   $E1"

E15=$(mk "E1.5 IR contract (linchpin)" \
  --type epic --priority high --status todo --parent "$MILESTONE" --tag critical-path \
  --body "Author & freeze docs/IR-CONTRACT.md. WRITE FIRST. Worked OpenSpec \
change exists at openspec/changes/define-ir-contract/ (validated). Blocks \
E2/E3/E3.5. OpenSpec changes: define-ir-contract.")
echo "  E1.5: $E15"

E4a=$(mk "E4a Fake tfprotov6 providers (alpha, beta)" \
  --type epic --priority high --status todo --parent "$MILESTONE" --tag critical-path \
  --body "Two in-repo providers with computed outputs; hermetic test substrate. \
Build early. Spec in docs/TESTING.md. OpenSpec changes: (record here).")
echo "  E4a:  $E4a"

E3=$(mk "E3 Go executor (terrae nivis)" \
  --type epic --priority high --status todo --parent "$MILESTONE" --tag critical-path \
  --body "IR ingestion, trivial JSON state, plugin manager (spawn, plain v6 \
handshake), DAG + TF->TF resolution, plan/apply, refresh/destroy/CLI. ROADMAP \
Epic 3. OpenSpec changes: (record here).")
echo "  E3:   $E3"

E35=$(mk "E3.5 Phased evaluation to fixpoint" \
  --type epic --priority high --status todo --parent "$MILESTONE" --tag critical-path \
  --body "THE THESIS. Outputs ledger, phase driver, fixpoint/cycle detection, \
*->Nix feedback. Generalizes 2-phase to N-phase. ROADMAP Epic 3.5. OpenSpec \
changes: (record here).")
echo "  E3.5: $E35"

E4b=$(mk "E4b Headline two-provider e2e (milestone exit)" \
  --type epic --priority high --status todo --parent "$MILESTONE" --tag critical-path \
  --body "Two providers, unknowns both sides, >=3 phases, Nix consumer reads \
both. Full spec docs/TESTING.md. This passing == milestone done. OpenSpec \
changes: (record here).")
echo "  E4b:  $E4b"

E2=$(mk "E2 Provider schema codegen (tn-gen)" \
  --type epic --priority normal --status todo --parent "$MILESTONE" --tag off-critical-path \
  --body "OFF critical path; build AFTER E4b. Schema->Nix type model + \
constructor codegen + override seam. Registry download is network-gated, \
separate bean. OpenSpec changes: (record here).")
echo "  E2:   $E2"

E4cd=$(mk "E4c/4d Error UX & docs" \
  --type epic --priority normal --status todo --parent "$MILESTONE" --tag off-critical-path \
  --body "Actionable errors with resource identity; README, getting-started on \
fake providers, IR contract & flake interface as documented contracts. \
OpenSpec changes: (record here).")
echo "  E4cd: $E4cd"

# --- Critical-path dependency edges (ROADMAP.md) ---------------------------
#   E1  ─┐
#        ├─> E1.5 ─> E4a ─> E3 ─> E3.5 ─> E4b
#   (E1 and E1.5 both feed the executor work; E4a is the test substrate.)
#   E2 and E4c/4d hang off E4b (built after the thesis is proven).
echo "Wiring critical-path blockers..."
beans update "$E15"  --blocked-by "$E1"  --json >/dev/null   # contract needs the lib's shape
beans update "$E4a"  --blocked-by "$E15" --json >/dev/null   # fakes target the frozen IR
beans update "$E3"   --blocked-by "$E15" --json >/dev/null   # executor ingests the IR
beans update "$E3"   --blocked-by "$E4a" --json >/dev/null   # executor tested against fakes
beans update "$E35"  --blocked-by "$E3"  --json >/dev/null   # phased loop drives the executor
beans update "$E4b"  --blocked-by "$E35" --json >/dev/null   # headline e2e needs the loop
beans update "$E2"   --blocked-by "$E4b" --json >/dev/null   # codegen is post-thesis breadth
beans update "$E4cd" --blocked-by "$E4b" --json >/dev/null   # docs/UX polish after exit

echo
echo "Done. Now:"
echo "  beans list --type epic                 # review all epics"
echo "  beans list --type epic --ready         # the next unblocked epic(s) to start"
echo "  beans roadmap                          # milestone/epic roadmap view"
echo "  Record OpenSpec change-ids in each epic's body as you create them."
