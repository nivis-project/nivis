#!/usr/bin/env bash
# coverage.sh [--profile <out.txt>]
#
# Measures Go test coverage across the WHOLE module and enforces a floor per
# package. Run by `nix flake check` (checks.coverage) and usable by hand.
#
# Why not plain `go test -cover`: that reports each package's coverage of its OWN
# statements only, so a package exercised through another package's tests reads as
# 0%. This runs the suite with `-coverpkg=./...` and merges the per-binary results
# with `go tool covdata`, which attributes coverage wherever it actually happened.
# (Concatenating `-coverprofile` files instead double-counts every block once per
# test binary and produces a meaninglessly low number.)
#
# THRESHOLDS ARE A RATCHET. The project's target is 70% overall / 80% on core
# packages. The floors below are today's measured baseline, rounded down. Raise
# them as coverage improves; never lower one to make a change pass. A package
# below its floor fails the gate and names itself.
#
# The floors are calibrated for the NIX BUILD SANDBOX, where this runs as
# checks.coverage. The sandbox has no `nix` binary, so tests/e2e skips itself
# there; on a developer machine with nix on PATH the same command reports HIGHER
# numbers (e.g. internal/phase 81.7% vs 73.9%, overall 67.9% vs 67.0%). Floors
# must pass in the stricter environment, which is the sandbox.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

PROFILE=""
if [[ "${1:-}" == "--profile" ]]; then
  PROFILE="${2:?--profile needs a path}"
fi

# --- what is measured -------------------------------------------------------
#
# EXCLUDED entirely (never counted in the overall number, never floored):
#   internal/tfplugin5, internal/tfplugin6  generated protobuf/gRPC stubs (~6300
#                                           statements of machine-written code;
#                                           including them drags the overall
#                                           number to ~27% and measures nothing)
#   cmd/provider-*                          the fake providers' main packages
#   internal/fakeprovider*                  the fake provider substrate itself
EXCLUDE_RE='^(internal/tfplugin[56]|cmd/provider-|internal/fakeprovider)'

# OVERALL floor, over everything not excluded. Target: 70. (Sandbox: 67.0%.)
OVERALL_FLOOR=67

# CORE packages: the executor's contract and engine. Target: 80.
CORE_FLOOR=79
CORE_PKGS='internal/ir internal/state internal/graph internal/plan internal/tfcodec'

# Everything else that is not excluded and not core.
OTHER_FLOOR=65

# Per-package exceptions: packages currently below OTHER_FLOOR, pinned at their
# own baseline so they cannot regress while the rest is pulled up.
declare -A EXCEPTIONS=(
  # A core package, but a big slice of the phase driver is only exercised by
  # tests/e2e, which skips in the sandbox: 73.9% there vs 81.7% with nix on PATH.
  # Floored at the sandbox baseline; it already clears the 80% core target when
  # the e2e tests actually run.
  [internal/phase]=73
  [internal/registry]=35
  [internal/provider/v5]=46
  [cmd/nivis]=38
  [cmd/nivistutor]=40
)

# --- measure ----------------------------------------------------------------
COVDIR="$(mktemp -d)"
MERGED="$(mktemp)"
trap 'rm -rf "$COVDIR" "$MERGED"' EXIT

echo "==> running the suite with cross-package coverage"
go test -coverpkg=./... ./... -args -test.gocoverdir="$COVDIR" >/dev/null
go tool covdata textfmt -i="$COVDIR" -o="$MERGED"
if [[ -n "$PROFILE" ]]; then
  cp "$MERGED" "$PROFILE"
  echo "    merged profile written to $PROFILE"
fi

# --- report and enforce -----------------------------------------------------
awk -v exclude_re="$EXCLUDE_RE" \
    -v overall_floor="$OVERALL_FLOOR" \
    -v core_floor="$CORE_FLOOR" \
    -v core_pkgs="$CORE_PKGS" \
    -v other_floor="$OTHER_FLOOR" \
    -v exceptions="$(for k in "${!EXCEPTIONS[@]}"; do printf '%s=%s ' "$k" "${EXCEPTIONS[$k]}"; done)" '
BEGIN {
  n = split(core_pkgs, c, " ");  for (i = 1; i <= n; i++) is_core[c[i]] = 1
  n = split(exceptions, e, " "); for (i = 1; i <= n; i++) { split(e[i], kv, "="); if (kv[1] != "") floor_of[kv[1]] = kv[2] }
}
/^mode:/ || /^$/ { next }
{
  # <pkgpath>/<file>.go:<block> <numStmt> <count>
  loc = $1; stmts = $2; count = $3
  sub(/:.*$/, "", loc)
  sub(/\/[^/]*$/, "", loc)
  sub(/^github\.com\/nivis-project\/nivis\//, "", loc)
  total[loc] += stmts
  if (count > 0) covered[loc] += stmts
  seen[loc] = 1
}
END {
  fails = 0
  printf "\n%-32s %7s %8s   %s\n", "PACKAGE", "COV", "STMTS", "FLOOR"
  printf "%s\n", "-------------------------------------------------------------------"
  # deterministic order: sort keys
  k = 0; for (p in seen) keys[++k] = p
  for (i = 1; i <= k; i++) for (j = i + 1; j <= k; j++) if (keys[j] < keys[i]) { t = keys[i]; keys[i] = keys[j]; keys[j] = t }

  for (i = 1; i <= k; i++) {
    p = keys[i]
    pct = total[p] > 0 ? 100 * covered[p] / total[p] : 0
    if (p ~ exclude_re) { printf "%-32s %6.1f%% %8d   (excluded)\n", p, pct, total[p]; continue }

    all_covered += covered[p]; all_total += total[p]

    fl = (p in floor_of) ? floor_of[p] : (is_core[p] ? core_floor : other_floor)
    tag = (p in floor_of) ? "baseline" : (is_core[p] ? "core" : "")
    marker = ""
    if (pct + 0.05 < fl) { marker = "  << BELOW"; fails++ }
    printf "%-32s %6.1f%% %8d   >=%d %s%s\n", p, pct, total[p], fl, tag, marker
  }

  overall = all_total > 0 ? 100 * all_covered / all_total : 0
  printf "%s\n", "-------------------------------------------------------------------"
  printf "%-32s %6.1f%% %8d   >=%d overall\n", "TOTAL (excluding generated)", overall, all_total, overall_floor
  if (overall + 0.05 < overall_floor) { printf "\ncoverage: overall %.1f%% is below the %d%% floor\n", overall, overall_floor; fails++ }
  if (fails > 0) { printf "\ncoverage: %d floor(s) breached — raise coverage, do not lower the floor\n", fails; exit 1 }
  printf "\ncoverage: ok (target is 70%% overall / 80%% core; floors are the ratchet, raise them as you go)\n"
}
' "$MERGED"
