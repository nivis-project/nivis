#!/usr/bin/env bash
# Milestone release-notes gate (docs/RELEASING.md, beans-tvin).
#
# When a milestone closes, its release notes (docs/releases/<slug>.md) should show
# what users can DO, drawn from the tutorials' verified examples, not just a
# changelog. This gate makes "the notes exist and are current" an objective,
# CI-checkable golden test: for every COMPLETED milestone in beans, the committed
# notes must regenerate IDENTICALLY from scripts/milestone-notes.sh (which pulls
# the milestone's epics + the CHANGELOG + the tutorials' release-note blocks).
#
# A missing or stale notes file fails, naming the milestone and the generator.
# Skips cleanly if `beans` is not available. Hermetic apart from reading beans.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "== milestone release-notes gate =="

if ! command -v beans >/dev/null 2>&1; then
  echo "   note: beans not available — skipping milestone-notes check"
  exit 0
fi

# Milestones that closed BEFORE this gate existed are exempt: their release was
# the corresponding CHANGELOG entry, and the tutorials' release-note markers are
# about later features, so generating notes would be anachronistic. The PoC
# (nixform2-hj4w, shipped as 0.1.0) is the only such milestone.
is_exempt() {
  case "$1" in
  nixform2-hj4w) return 0 ;;
  *) return 1 ;;
  esac
}

# completed milestone ids (+ a stable slug from the title), via beans.
# The slug rule MUST match scripts/milestone-notes.sh slugify(): strip a leading
# "M<n>:" prefix and any "(parenthetical)", then kebab-case.
mapfile -t MILESTONES < <(
  beans list --json 2>/dev/null | python3 -c '
import sys, json, re
d = json.load(sys.stdin)
bs = d if isinstance(d, list) else (d.get("beans") or d.get("data") or [])
def slugify(title):
    t = re.sub(r"^\s*M\d+\s*:\s*", "", title)
    t = re.sub(r"\s*\(.*?\)\s*", " ", t)
    t = t.split(":")[0]
    return re.sub(r"[^a-z0-9]+", "-", t.strip().lower()).strip("-")
for b in bs:
    if b.get("type") == "milestone" and b.get("status") == "completed":
        slug = slugify(b["title"])
        print(b["id"], slug)
'
)

fail=0
checked=0
for entry in "${MILESTONES[@]}"; do
  [ -z "$entry" ] && continue
  id="${entry%% *}"
  slug="${entry##* }"
  if is_exempt "$id"; then
    continue
  fi
  notes="docs/releases/${slug}.md"
  checked=$((checked + 1))

  if [ ! -f "$notes" ]; then
    echo "FAIL: completed milestone $id has no release notes ($notes)"
    echo "      generate them: scripts/milestone-notes.sh $id"
    fail=1
    continue
  fi

  tmp="$(mktemp)"
  if ! bash scripts/milestone-notes.sh "$id" --stdout >"$tmp" 2>/dev/null; then
    echo "FAIL: could not regenerate notes for $id"
    rm -f "$tmp"
    fail=1
    continue
  fi
  if ! diff -q "$notes" "$tmp" >/dev/null 2>&1; then
    echo "FAIL: $notes is stale (regenerates differently)"
    echo "      refresh it: scripts/milestone-notes.sh $id"
    fail=1
  fi
  rm -f "$tmp"
done

if [ "$fail" -eq 0 ]; then
  echo "   ok: all $checked completed milestone(s) have current release notes"
fi
exit "$fail"
