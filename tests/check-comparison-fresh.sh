#!/usr/bin/env bash
# Comparison-page freshness check (beans-0bll).
#
# docs/COMPARISON.md compares Nivis to OTHER tools using facts about those tools
# (their versions, features, licenses) that drift over time. Unlike the rest of
# our docs, it can rot without anyone touching this repo. This guard makes the rot
# visible: the page carries a `last-verified: YYYY-MM-DD` marker, and this script
# fails once that date is older than MAX_AGE_DAYS, prompting a human to re-confirm
# the claims against the page's "Sources" section and bump the date.
#
# It also asserts the page still has the structural bits the mechanism relies on
# (the marker, the Sources section). Hermetic: no network; date math only.
#
# Override the window with: MAX_AGE_DAYS=90 bash tests/check-comparison-fresh.sh
set -euo pipefail
cd "$(dirname "$0")/.."

PAGE="docs/COMPARISON.md"
MAX_AGE_DAYS="${MAX_AGE_DAYS:-180}"
fail=0

echo "== comparison page freshness =="

if [ ! -f "$PAGE" ]; then
  echo "FAIL: $PAGE is missing"
  exit 1
fi

# 1. The page must carry the freshness marker.
verified="$(grep -oE 'last-verified: [0-9]{4}-[0-9]{2}-[0-9]{2}' "$PAGE" | head -n1 | awk '{print $2}')"
if [ -z "$verified" ]; then
  echo "FAIL: no 'last-verified: YYYY-MM-DD' marker found in $PAGE"
  echo "      add one (in an HTML comment near the top) and keep it current."
  exit 1
fi

# 2. The Sources section must exist (that's what a re-verifier checks against).
if ! grep -qiE '^##+ *Sources' "$PAGE"; then
  echo "FAIL: $PAGE has no '## Sources' section to re-verify against"
  fail=1
fi

# 3. The marker must parse as a real date and not be in the future.
if ! verified_epoch="$(date -u -d "$verified" +%s 2>/dev/null)"; then
  echo "FAIL: last-verified date '$verified' is not a valid date"
  exit 1
fi
now_epoch="$(date -u +%s)"
if [ "$verified_epoch" -gt "$now_epoch" ]; then
  echo "FAIL: last-verified date '$verified' is in the future"
  fail=1
fi

# 4. Age check.
age_days=$(( (now_epoch - verified_epoch) / 86400 ))
if [ "$age_days" -gt "$MAX_AGE_DAYS" ]; then
  echo "FAIL: comparison is stale — last verified $verified ($age_days days ago, max ${MAX_AGE_DAYS})."
  echo "      re-check the claims against the Sources in $PAGE, then bump 'last-verified'."
  fail=1
else
  echo "   ok: last verified $verified ($age_days days ago, within ${MAX_AGE_DAYS})."
fi

exit "$fail"
