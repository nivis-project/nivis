#!/usr/bin/env bash
# Cut a release: bump VERSION (the single source of truth), roll the changelog's
# Unreleased section into a dated version section, commit, tag v<version>, and
# push the tag. The tag push triggers .github/workflows/release.yml (goreleaser),
# which builds the binaries and creates the GitHub release.
#
# Usage:
#   scripts/release.sh patch|minor|major [--dry-run]
#
# jj/git compatible: commits via jj when this is a jj repo, else git; the tag is
# always created on the git (backing) repo so it pushes to GitHub.
set -euo pipefail
cd "$(dirname "$0")/.."

bump="${1:-}"
dry=0
[ "${2:-}" = "--dry-run" ] && dry=1
case "$bump" in
  patch | minor | major) ;;
  *)
    echo "usage: scripts/release.sh patch|minor|major [--dry-run]" >&2
    exit 2
    ;;
esac

[ -f VERSION ] || { echo "VERSION file missing" >&2; exit 1; }
cur="$(tr -d '[:space:]' < VERSION)"
IFS=. read -r major minor patch <<<"$cur"
if [[ -z "$major" || -z "$minor" || -z "$patch" ]]; then
  echo "VERSION is not semver (got '$cur')" >&2
  exit 1
fi
case "$bump" in
  major) major=$((major + 1)); minor=0; patch=0 ;;
  minor) minor=$((minor + 1)); patch=0 ;;
  patch) patch=$((patch + 1)) ;;
esac
next="${major}.${minor}.${patch}"
tag="v${next}"
today="$(date +%Y-%m-%d)"
repo="https://github.com/wearetechnative/terrae-nivis"

echo "release: ${cur} -> ${next}  (tag ${tag}, ${today})"

if [ "$dry" -eq 1 ]; then
  echo "[dry-run] would write VERSION=${next}"
  echo "[dry-run] would roll CHANGELOG 'Unreleased' into '## [${next}] - ${today}'"
  echo "[dry-run] would commit, create tag ${tag}, and push the tag"
  exit 0
fi

# 1. VERSION
printf '%s\n' "$next" > VERSION

# 2. CHANGELOG: insert a dated section under Unreleased, refresh the link refs.
if [ -f CHANGELOG.md ]; then
  python3 - "$next" "$today" "$repo" <<'PY'
import sys, re
nxt, today, repo = sys.argv[1], sys.argv[2], sys.argv[3]
p = "CHANGELOG.md"
s = open(p).read()

# Insert a new dated section immediately after the "## [Unreleased]" heading,
# leaving Unreleased in place (empty) for the next cycle.
marker = "## [Unreleased]"
i = s.index(marker) + len(marker)
s = s[:i] + f"\n\n## [{nxt}] - {today}" + s[i:]

# Refresh link refs: point Unreleased at <tag>...HEAD and add the new tag link.
s = re.sub(r"\[Unreleased\]:.*", f"[Unreleased]: {repo}/compare/v{nxt}...HEAD", s, count=1)
if f"[{nxt}]:" not in s:
    # add the new version link right after the Unreleased link line
    s = re.sub(r"(\[Unreleased\]:.*\n)", rf"\1[{nxt}]: {repo}/releases/tag/v{nxt}\n", s, count=1)
open(p, "w").write(s)
print(f"changelog: added [{nxt}] - {today}")
PY
fi

# 3. Commit (jj if present, else git).
msg="Release ${tag}"
if command -v jj >/dev/null 2>&1 && [ -d .jj ]; then
  printf '%s\n' "$msg" | jj describe --stdin
  jj bookmark set main -r @ >/dev/null 2>&1 || true
  jj new >/dev/null
else
  git add VERSION CHANGELOG.md
  git commit -m "$msg"
fi

# 4. Tag on the git backing repo (so it pushes to GitHub) and push the tag.
git tag -a "$tag" -m "$msg"
git push origin "$tag"

echo "done: pushed ${tag}. The release workflow will build and publish it."
