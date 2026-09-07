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
# git remains the backing store and the push transport.
#
# The OpenSpec store this project points at (openspec/config.yaml `store:`) is a
# separate repo, shared with the org's other projects. Committing it is NOT done
# here: the store ships its own scripts/store-commit.sh (fetch + rebase + commit
# + push, with an honest message) and scripts/store-hygiene.sh (the gate), so
# every client project treats the store identically and a concurrent push from
# another project cannot strand this one. See the store's README, "The hygiene
# contract for a client project".
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
echo "==> [1/6] stage working tree (so nix flake sees new files)"
git add -A

echo "==> [2/6] gate"
echo "--> nix flake check (build, tests, coverage floors, gofmt)"
nix flake check
echo "--> Nix-library tests"
bash tests/run-nix-tests.sh
echo "--> docs checks"
bash tests/check-docs-ssot.sh

echo "==> [3/6] archive OpenSpec change: ${CHANGE}"
openspec archive "${CHANGE}" "${STORE_FLAG[@]}" --yes

echo "==> [4/6] commit"
git add -A
jj commit -m "${SUBJECT}"

# The store goes out BEFORE this repo is published. The store push is the one
# that can be rejected (other projects push there too), and a change whose code
# is public while its archived record is stranded on one laptop is the worse of
# the two failure shapes.
echo "==> [5/6] commit and push the OpenSpec store"
if [[ -z "$STORE_ROOT" ]]; then
  echo "    (changes are repo-local; nothing to do)"
else
  if [[ -x "${STORE_ROOT}/scripts/store-commit.sh" ]]; then
    bash "${STORE_ROOT}/scripts/store-commit.sh" "Archive ${CHANGE}"
  else
    # An older store checkout without the helpers: do it inline, and say so, so
    # the gap is visible rather than silently unprotected.
    echo "    note: ${STORE_ROOT} has no scripts/store-commit.sh (old checkout?);" >&2
    echo "          committing inline without the fetch/rebase protection" >&2
    git -C "$STORE_ROOT" add -A
    git -C "$STORE_ROOT" commit -m "Archive ${CHANGE}"
    git -C "$STORE_ROOT" push origin main
  fi

  # The post-condition: after a finished ship, nothing about this change may
  # exist only on this machine.
  if [[ -x "${STORE_ROOT}/scripts/store-hygiene.sh" ]]; then
    bash "${STORE_ROOT}/scripts/store-hygiene.sh"
  fi
fi

echo "==> [6/6] push main"
# `jj commit` leaves a new empty working-copy commit, so the change just made is
# @- — move the bookmark there and push it.
jj bookmark set main -r @-
jj git push --bookmark main

echo "==> shipped ${CHANGE}"
