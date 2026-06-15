#!/usr/bin/env bash
# Docs single-source-of-truth check (beans-qvx3 / openspec change docs-ssot).
#
# Each shared documentation topic has ONE canonical file; the mdBook site
# {{#include}}s it and README links to it — nothing copies it. This guards
# against re-introducing the duplication we removed: a distinctive "fingerprint"
# line for each canonical block must appear in exactly its canonical file and
# NOT be pasted into README or the site's framing pages.
#
# It also verifies the site still builds with the includes (skipped, with a
# notice, if mdbook is not installed — the duplication check still runs).
#
# Exits non-zero on any violation. Hermetic: no network.
set -euo pipefail
cd "$(dirname "$0")/.."

fail=0

# A fingerprint is a verbatim phrase from a canonical block. The block lives in
# CANON; it must not appear in any of the OTHER (framing/link) files.
#   check_unique "<fingerprint>" "<canonical file>" "<file that must NOT copy it>"...
check_unique() {
  local needle="$1" canon="$2"; shift 2
  if ! grep -qF -- "$needle" "$canon"; then
    echo "FAIL: canonical block missing from $canon"
    echo "      expected to find: $needle"
    fail=1
    return
  fi
  local f
  for f in "$@"; do
    [ -f "$f" ] || continue
    if grep -qF -- "$needle" "$f"; then
      echo "FAIL: duplicated canonical block (should link/include, not copy)"
      echo "      canonical : $canon"
      echo "      copied in : $f"
      echo "      fingerprint: $needle"
      fail=1
    fi
  done
}

echo "== docs single-source-of-truth =="

# 1. "How it works" prose — canonical in docs/OVERVIEW.md; README links, site includes.
check_unique \
  "collects the" \
  docs/OVERVIEW.md \
  README.md docs-site/src/index.md

# 2. The "round trip" pitch — canonical in docs/OVERVIEW.md.
#    (README keeps a short headline paragraph but not this exact sentence.)
check_unique \
  "unknown values originating on both sides" \
  docs/OVERVIEW.md \
  docs-site/src/index.md

# 3. The full AWS plan/apply/state/destroy walkthrough — canonical in the
#    step-by-step tutorial. getting-started §7 only links to it; README only
#    links; the site's real-providers.md includes §7 (the link), not the commands.
check_unique \
  "tn state show aws.aws_s3_bucket.demo" \
  docs/TUTORIAL-AWS-S3.md \
  README.md docs/GETTING-STARTED.md docs-site/src/real-providers.md

# 4. The fake-provider build/run command block — canonical in getting-started;
#    README's quickstart is a shorter, distinct set (no provider-alpha+beta+tn trio
#    followed by the four-command plan/apply/state/destroy walkthrough).
check_unique \
  "tears down in reverse dependency order" \
  docs/GETTING-STARTED.md \
  README.md docs-site/src/index.md

# 5. Install instructions — canonical in docs/INSTALL.md; the tutorial and README
#    link to it rather than repeating the persistent-install command.
check_unique \
  "nix profile install github:wearetechnative/terrae-nivis#tn" \
  docs/INSTALL.md \
  docs/TUTORIAL-AWS-S3.md README.md

if [ "$fail" -eq 0 ]; then
  echo "   ok: no canonical block is duplicated"
else
  echo "   ^ fix by replacing the copy with a link (README) or an {{#include}} (site)."
fi

# 5. The site must still build with the includes in place.
echo "== site builds with includes =="
if command -v mdbook >/dev/null 2>&1; then
  if mdbook build docs-site >/dev/null 2>&1; then
    # No include directive may survive into the rendered HTML.
    if grep -rlq "{{#include" docs-site/book/*.html 2>/dev/null; then
      echo "FAIL: an unrendered {{#include}} leaked into the built site"
      fail=1
    else
      echo "   ok: mdbook build succeeded, all includes resolved"
    fi
  else
    echo "FAIL: mdbook build docs-site failed"
    fail=1
  fi
else
  echo "   note: mdbook not installed — skipping the build check (run: cargo install mdbook)"
fi

exit "$fail"
