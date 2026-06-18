#!/usr/bin/env bash
# Changelog-update gate (docs/RELEASING.md, beans-gyea).
#
# A user-facing change should not ship without a changelog note. This ties the
# changelog to OpenSpec archival: every archived change's proposal.md declares a
#   Changelog: <entry text>     (added under ## [Unreleased] in CHANGELOG.md)
# or
#   Changelog: none - <reason>  (internal / non-user-facing change)
# and this gate enforces that the line exists and that a non-`none` entry is
# actually present in CHANGELOG.md's [Unreleased] section. It does not judge
# wording; it ensures the call was made and a declared entry is present before a
# release rolls [Unreleased] into a version.
#
# Changes archived BEFORE this convention (by date) are exempt. Hermetic: reads
# files only. Exits non-zero on a violation.
set -euo pipefail
cd "$(dirname "$0")/.."

# Archives dated on/before this predate the convention and are exempt.
CUTOFF="2026-06-18"

fail=0
checked=0

echo "== changelog-update gate =="

# normalize: drop markdown markup (* _ ` # -) and collapse all whitespace to
# single spaces, lowercased. Makes the substring check tolerant of bold/code
# formatting and line wrapping in the changelog and the declared entry.
normalize() { tr -d '*_`#' | tr '\n' ' ' | tr -s ' ' | tr '[:upper:]' '[:lower:]'; }

# Extract + normalize the current ## [Unreleased] section of CHANGELOG.md
# (up to the next "## " header), for substring checks.
unreleased="$(awk '
  /^## \[Unreleased\]/ {grab=1; next}
  /^## / && grab {exit}
  grab {print}
' CHANGELOG.md | normalize)"

shopt -s nullglob
for p in openspec/changes/archive/*/proposal.md; do
  dir="$(basename "$(dirname "$p")")"
  # exempt pre-convention archives by date prefix (YYYY-MM-DD-...)
  if [[ "$dir" =~ ^([0-9]{4}-[0-9]{2}-[0-9]{2})- ]]; then
    date="${BASH_REMATCH[1]}"
    if [[ "$date" < "$CUTOFF" || "$date" == "$CUTOFF" ]]; then
      continue
    fi
  fi
  checked=$((checked + 1))

  # the Changelog: line value (everything after the first colon, trimmed).
  line="$(grep -iE '^[[:space:]]*Changelog:' "$p" | head -n1 || true)"
  if [ -z "$line" ]; then
    echo "FAIL: $p has no 'Changelog:' line"
    echo "      add one: 'Changelog: <entry>' or 'Changelog: none - <why>'"
    fail=1
    continue
  fi
  value="$(printf '%s' "$line" | sed -E 's/^[[:space:]]*[Cc]hangelog:[[:space:]]*//')"

  # `none` (with or without a reason) needs no changelog entry.
  if [[ "$value" =~ ^none([[:space:]].*)?$ ]] || [[ "$value" =~ ^none- ]]; then
    continue
  fi

  # Otherwise a fingerprint of the entry must appear in [Unreleased]. Normalize
  # the declared value the same way, and use its first ~5 words as a distinctive,
  # formatting- and wrapping-tolerant fingerprint.
  fingerprint="$(printf '%s' "$value" | normalize | cut -d' ' -f1-5)"
  if ! printf '%s' "$unreleased" | grep -qF -- "$fingerprint"; then
    echo "FAIL: $dir declares a changelog entry not found in CHANGELOG.md [Unreleased]"
    echo "      declared: $value"
    echo "      add it under '## [Unreleased]' (normalized fingerprint: \"$fingerprint\")"
    fail=1
  fi
done

if [ "$fail" -eq 0 ]; then
  echo "   ok: all $checked post-cutoff archived change(s) declare a changelog status"
fi
exit "$fail"
