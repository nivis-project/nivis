#!/usr/bin/env bash
# Documentation-coverage gate (docs/DOCS-GATE.md).
#
# As features grow, "did the docs keep up?" must be a checkable step, not a thing
# an author remembers. Whether a change needs a NEW DOCUMENT, an EXTRA PARAGRAPH,
# MODIFICATIONS, or NOTHING is a judgment a regex cannot make, so this script does
# NOT judge content. It enforces only that the judgment was RECORDED: every
# OpenSpec change's proposal.md must carry a `Docs impact:` line. The author (a
# human or an agent) makes the call per the rubric in docs/DOCS-GATE.md; this
# guarantees the call was made and written down, never silently skipped.
#
# Reads files only, no network. Exits non-zero on any change missing the note
# (excluding pre-gate changes, which are exempt by date).
#
# The changes live in a central OpenSpec store shared with other projects;
# tests/lib/openspec-scope.sh resolves that root and filters to the capabilities
# this repository owns. Where the store is unreachable (CI), this gate says so
# rather than reporting a vacuous pass.
set -euo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=tests/lib/openspec-scope.sh
source tests/lib/openspec-scope.sh

# Changes archived on or before this date predate the gate and are exempt. New
# changes (active, or archived after this date) must carry the note.
GATE_CUTOFF="2026-06-16"

fail=0
checked=0

echo "== documentation-coverage gate =="

root="$(openspec_root)"
if [ -z "$root" ]; then
  openspec_unreachable_notice
  exit 0
fi

# Collect proposal.md files for active changes and archived changes.
#   active:   <root>/openspec/changes/<id>/proposal.md
#   archived: <root>/openspec/changes/archive/<YYYY-MM-DD-id>/proposal.md
shopt -s nullglob
proposals=("$root"/openspec/changes/*/proposal.md "$root"/openspec/changes/archive/*/proposal.md)
shopt -u nullglob

skipped=0
for p in "${proposals[@]}"; do
  changedir="$(dirname "$p")"
  dir="$(basename "$changedir")"

  # Another project's change in the shared store: not ours to annotate.
  if ! change_is_ours "$changedir"; then
    skipped=$((skipped + 1))
    continue
  fi

  # Exempt pre-gate archived changes by their date prefix (YYYY-MM-DD-...).
  if [[ "$dir" =~ ^([0-9]{4}-[0-9]{2}-[0-9]{2})- ]]; then
    date="${BASH_REMATCH[1]}"
    # string compare works for ISO dates
    if [[ "$date" < "$GATE_CUTOFF" || "$date" == "$GATE_CUTOFF" ]]; then
      continue
    fi
  fi

  checked=$((checked + 1))
  if ! grep -qiE '^[[:space:]]*Docs impact:' "$p"; then
    echo "FAIL: $p has no 'Docs impact:' line"
    echo "      decide per docs/DOCS-GATE.md (new doc / paragraph / edits / none)"
    echo "      and record it, e.g.: 'Docs impact: new document — docs/X.md ...'"
    fail=1
  fi
done

if [ "$fail" -eq 0 ]; then
  echo "   ok: all $checked in-scope change(s) record a 'Docs impact:' decision"
  [ "$skipped" -gt 0 ] && echo "       ($skipped change(s) in the shared store belong to other projects)"
fi

exit "$fail"
