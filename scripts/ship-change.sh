#!/usr/bin/env bash
# ship-change.sh <change-name> [commit-subject]
#
# Gated tail for shipping ONE implemented OpenSpec change:
#   stage -> gate (nix flake check) -> archive -> commit -> push.
# If the gate fails this aborts before archiving or committing.
#
# The change may live in a central OpenSpec store (openspec/config.yaml's
# `store:` key) rather than in this repo. When it does, the archive lands in the
# store's own git repo, so this script commits and pushes that repo too.
set -euo pipefail

CHANGE="${1:?usage: ship-change.sh <change-name> [commit-subject]}"
SUBJECT="${2:-Implement ${CHANGE}}"

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

# The OpenSpec store this project points at, if any (empty = repo-local).
STORE="$(sed -n 's/^store:[[:space:]]*//p' openspec/config.yaml 2>/dev/null | head -1)"
STORE_FLAG=()
CHANGE_DIR="openspec/changes/${CHANGE}"
STORE_ROOT=""
if [[ -n "$STORE" ]]; then
  STORE_FLAG=(--store "$STORE")
  STORE_ROOT="$(openspec store list --json | python3 -c \
    "import json,sys; print(next(s['root'] for s in json.load(sys.stdin)['stores'] if s['id']=='$STORE'))")"
  CHANGE_DIR="${STORE_ROOT}/openspec/changes/${CHANGE}"
fi

TASKS="${CHANGE_DIR}/tasks.md"
if [[ ! -d "$CHANGE_DIR" ]]; then
  echo "ship: no active change ${CHANGE} at ${CHANGE_DIR}" >&2
  exit 1
fi
if [[ -f "$TASKS" ]] && grep -qE "^\s*- \[ \]" "$TASKS"; then
  echo "ship: $TASKS still has unchecked tasks — finish the apply step first" >&2
  exit 1
fi

echo "==> [1/5] stage working tree (so nix flake sees new files)"
git add -A

echo "==> [2/5] gate: nix flake check"
nix flake check

echo "==> [3/5] archive OpenSpec change: ${CHANGE}"
openspec archive "${CHANGE}" "${STORE_FLAG[@]}" --yes

echo "==> [4/5] commit"
git add -A
git commit -m "${SUBJECT}"
if [[ -n "$STORE_ROOT" ]]; then
  git -C "$STORE_ROOT" add -A
  git -C "$STORE_ROOT" commit -m "Archive ${CHANGE}"
fi

echo "==> [5/5] push main"
git push origin main
if [[ -n "$STORE_ROOT" ]]; then
  git -C "$STORE_ROOT" push origin main
fi

echo "==> shipped ${CHANGE}"
