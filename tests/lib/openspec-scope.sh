#!/usr/bin/env bash
# Where this project's OpenSpec changes live, and which of them are ours.
#
# Sourced by tests/check-docs-gate.sh and tests/check-changelog.sh. Both gates
# read `proposal.md` files; both used to glob `openspec/changes/` inside this
# repository. Since this project moved to a CENTRAL OpenSpec store
# (openspec/config.yaml `store:`), that directory is empty — which is exactly how
# the two gates went blind: they found nothing and cheerfully reported
# "all 0 in-scope change(s)".
#
# Two problems have to be solved together:
#
#   1. FIND the changes. They live in the store, whose path is machine-local
#      (`openspec store list`). $OPENSPEC_ROOT overrides it. When a store is
#      declared but unreachable — CI, where the private store repo is not checked
#      out — the gates MUST say they are not enforcing, never report success.
#
#   2. FILTER to ours. The store is SHARED with other projects (registry-ui-fork,
#      nivis-demos). `Docs impact:` / `Changelog:` are THIS repository's
#      conventions, referring to this repository's docs/ and CHANGELOG.md, so
#      enforcing them over another project's changes would fail on work that is
#      not ours to annotate.
set -euo pipefail

# The capabilities (specs/<name>/) this repository owns. Derived from which
# changes in the store touch which capability; the remainder belong to
# registry-ui-fork (compat-tiers, dev-environment, docs-contract, e2e-proof,
# full-catalog-extraction, nix-rendering, project-scaffold, provider-extraction,
# registry-ui-fork, seed-selection) and nivis-demos (demos-*). Add a capability
# here when this repo starts owning one.
OWNED_CAPABILITIES=(
  branding
  cli
  codegen
  e2e
  executor
  fake-providers
  ir
  nix-lib
  registry
  release
)

# openspec_declared_store prints the store id from openspec/config.yaml (empty if
# the project keeps its changes locally).
openspec_declared_store() {
  sed -n 's/^store:[[:space:]]*//p' openspec/config.yaml 2>/dev/null | head -1
}

# openspec_root prints the directory holding `openspec/changes` for this project,
# or nothing when a store is declared but cannot be reached here.
openspec_root() {
  if [ -n "${OPENSPEC_ROOT:-}" ]; then
    # An override that does not actually hold a changes tree would produce a
    # vacuous pass, which is the failure mode this file exists to prevent.
    if [ -d "$OPENSPEC_ROOT/openspec/changes" ]; then
      printf '%s\n' "$OPENSPEC_ROOT"
    fi
    return 0
  fi
  local store root
  store="$(openspec_declared_store)"
  if [ -z "$store" ]; then
    printf '.\n' # no store: the changes are in this repo
    return 0
  fi
  command -v openspec >/dev/null 2>&1 || return 0 # unreachable: print nothing
  command -v python3 >/dev/null 2>&1 || return 0
  root="$(openspec store list --json 2>/dev/null |
    python3 -c "import json,sys
try:
    d = json.load(sys.stdin)
except Exception:
    raise SystemExit(0)
for s in d.get('stores', []):
    if s.get('id') == '$store':
        print(s.get('root', ''))
        break" 2>/dev/null)" || return 0
  [ -n "$root" ] && [ -d "$root/openspec/changes" ] && printf '%s\n' "$root"
  return 0
}

# openspec_unreachable_notice prints the "not enforcing" explanation for a gate
# that could not find the declared store, so the blindness is visible in the log
# instead of masquerading as a pass.
openspec_unreachable_notice() {
  local store
  store="$(openspec_declared_store)"
  echo "   NOT ENFORCED: this project's changes live in the OpenSpec store '${store}',"
  echo "                 which is not available here (the store is a separate, private repo)."
  echo "                 Run this gate on a machine with the store, or set"
  echo "                 OPENSPEC_ROOT=<path to a store checkout> to enforce it."
}

# change_is_ours <change-dir> succeeds when the change touches a capability this
# repository owns. A change with no specs/ at all cannot be attributed, so it is
# treated as ours (fail loud rather than skip silently) — see the note printed by
# the caller.
change_is_ours() {
  local dir="$1" cap
  if [ ! -d "$dir/specs" ]; then
    return 0
  fi
  for cap in "${OWNED_CAPABILITIES[@]}"; do
    [ -d "$dir/specs/$cap" ] && return 0
  done
  return 1
}
