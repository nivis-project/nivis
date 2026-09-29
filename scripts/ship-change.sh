#!/usr/bin/env bash
# ship-change.sh <change-name> [commit-subject] [--bean <id>]...
#
# Gated tail for shipping ONE implemented OpenSpec change:
#   stage -> gate -> archive -> close bean(s) -> commit -> push.
# If any gate fails this aborts before archiving, closing anything, or committing.
#
# --bean closes a bean as part of the ship, so a ship is ONE commit. Closing it
# afterwards, from outside, produced a second commit seconds later, and CI
# (concurrency: cancel-in-progress) then cancelled the first run — leaving the
# implementation commit with no completed CI run of its own. A bean closure is not
# independent work; it is part of shipping, so it belongs in the same commit.
#
# Write the bean's `## Summary of Changes` BEFORE running this (it is prose only
# the author can write); this script flips the status, after the gate, so a failed
# gate leaves the bean untouched.
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

CHANGE="${1:?usage: ship-change.sh <change-name> [commit-subject] [--bean <id>]...}"
shift
SUBJECT="Implement ${CHANGE}"
if [[ $# -gt 0 && "$1" != --* ]]; then
  SUBJECT="$1"
  shift
fi
BEANS=()
while [[ $# -gt 0 ]]; do
  case "$1" in
  --bean)
    BEANS+=("${2:?--bean needs a bean id}")
    shift 2
    ;;
  *)
    echo "ship: unknown argument $1" >&2
    exit 1
    ;;
  esac
done

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

# The OpenSpec store this project points at, if any (empty = repo-local).
STORE="$(sed -n 's/^store:[[:space:]]*//p' openspec/config.yaml 2>/dev/null | head -1)"
STORE_FLAG=()
CHANGE_DIR="openspec/changes/${CHANGE}"
STORE_ROOT=""
STORE_REPO=""
if [[ -n "$STORE" ]]; then
  STORE_FLAG=(--store "$STORE")
  STORE_ROOT="$(openspec store list --json | python3 -c \
    "import json,sys; print(next(s['root'] for s in json.load(sys.stdin)['stores'] if s['id']=='$STORE'))")"
  CHANGE_DIR="${STORE_ROOT}/openspec/changes/${CHANGE}"
  # The helper scripts live at the store REPO's root, NOT at the store's own
  # root: one working tree holds several stores as subdirectories, and
  # scripts/ sits one level above any of them.
  STORE_REPO="$(git -C "$STORE_ROOT" rev-parse --show-toplevel)"
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
echo "==> [1/7] stage working tree (so nix flake sees new files)"
git add -A

echo "==> [2/7] gate"
echo "--> nix flake check (build, tests, coverage floors, gofmt)"
nix flake check
echo "--> Nix-library tests"
bash tests/run-nix-tests.sh
echo "--> docs checks"
bash tests/check-docs-ssot.sh

echo "==> [3/7] archive OpenSpec change: ${CHANGE}"
openspec archive "${CHANGE}" "${STORE_FLAG[@]}" --yes

echo "==> [4/7] close the bean(s)"
if [[ ${#BEANS[@]} -eq 0 ]]; then
  echo "    (no --bean given; nothing to close)"
else
  for bean in "${BEANS[@]}"; do
    # The summary is the author's prose, not this script's: warn rather than fail,
    # since by here the gate has passed and the change is archived.
    file="$(find .beans -maxdepth 1 -name "${bean}--*.md" -print -quit 2>/dev/null || true)"
    if [[ -n "$file" ]] && ! grep -q "^## Summary of Changes" "$file"; then
      echo "    warning: $file has no '## Summary of Changes' section" >&2
    fi
    beans update "$bean" -s completed
  done
  # A bean whose parent milestone is now fully complete is a judgement call
  # (which siblings count?), so closing a milestone stays the author's job.
fi

echo "==> [5/7] commit"
git add -A
jj commit -m "${SUBJECT}"

# The store goes out BEFORE this repo is published. The store push is the one
# that can be rejected (other projects push there too), and a change whose code
# is public while its archived record is stranded on one laptop is the worse of
# the two failure shapes.
echo "==> [6/7] commit and push the OpenSpec store"
if [[ -z "$STORE_ROOT" ]]; then
  echo "    (changes are repo-local; nothing to do)"
else
  if [[ -x "${STORE_REPO}/scripts/store-commit.sh" ]]; then
    bash "${STORE_REPO}/scripts/store-commit.sh" "Archive ${CHANGE}"
  else
    # There used to be an inline fallback here. It committed with plain git,
    # which in a jj-colocated store lands on a DETACHED HEAD, and then pushed
    # `main`, which had not moved: "Everything up-to-date", exit 0, archive
    # stranded on the laptop while the code went public. A wrong result that
    # looks like success is worse than no fallback, so this aborts instead.
    echo "ship: ${STORE_REPO}/scripts/store-commit.sh is missing or not executable." >&2
    echo "      The change is archived locally but NOT published. Update the store" >&2
    echo "      checkout, then commit and push it from ${STORE_REPO}." >&2
    exit 1
  fi

  # The post-condition: after a finished ship, nothing about this change may
  # exist only on this machine.
  if [[ -x "${STORE_REPO}/scripts/store-hygiene.sh" ]]; then
    bash "${STORE_REPO}/scripts/store-hygiene.sh"
  else
    echo "    warning: no store-hygiene.sh; the published-everything check did not run" >&2
  fi
fi

echo "==> [7/7] push main"
# `jj commit` leaves a new empty working-copy commit, so the change just made is
# @- — move the bookmark there and push it.
jj bookmark set main -r @-
jj git push --bookmark main

echo "==> shipped ${CHANGE}"
