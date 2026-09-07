#!/usr/bin/env bash
# ship-change.sh <change-name> [commit-subject]
#
# Gated tail for shipping ONE implemented OpenSpec change:
#   stage -> gate -> archive -> commit -> push.
# If any gate fails this aborts before archiving or committing.
#
# The gate is three parts:
#   1. `nix flake check`          build, `go test ./...`, coverage floors, gofmt
#   2. tests/run-nix-tests.sh     the Nix-library tests (needs `nix eval`, so it
#                                 cannot run inside the flake's build sandbox)
#   3. tests/check-docs-ssot.sh   docs single-source-of-truth + coverage gate
#
# VCS: this repo is **jj (Jujutsu), colocated with git**. jj drives the commit;
# git remains the backing store and the push transport. The OpenSpec store this
# project points at (openspec/config.yaml `store:`) is a separate, plain-git
# repo, so the archive commit there uses git.
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

# jj snapshots the working copy on its own, but `nix flake check` evaluates the
# GIT tree: a file git does not know about is invisible to the flake. Staging is
# how new files become visible. (The index is transient here; jj owns commits.)
echo "==> [1/5] stage working tree (so nix flake sees new files)"
git add -A

echo "==> [2/5] gate"
echo "--> nix flake check (build, tests, coverage floors, gofmt)"
nix flake check
echo "--> Nix-library tests"
bash tests/run-nix-tests.sh
echo "--> docs checks"
bash tests/check-docs-ssot.sh

echo "==> [3/5] archive OpenSpec change: ${CHANGE}"
openspec archive "${CHANGE}" "${STORE_FLAG[@]}" --yes

echo "==> [4/5] commit"
git add -A
jj commit -m "${SUBJECT}"
if [[ -n "$STORE_ROOT" ]]; then
  # The store is a plain git repo.
  git -C "$STORE_ROOT" add -A
  git -C "$STORE_ROOT" commit -m "Archive ${CHANGE}"
fi

echo "==> [5/5] push main"
# `jj commit` leaves a new empty working-copy commit, so the change just made is
# @- — move the bookmark there and push it.
jj bookmark set main -r @-
jj git push --bookmark main
if [[ -n "$STORE_ROOT" ]]; then
  git -C "$STORE_ROOT" push origin main
fi

echo "==> shipped ${CHANGE}"
