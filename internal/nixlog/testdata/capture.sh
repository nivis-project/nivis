#!/usr/bin/env bash
# Regenerate the `--log-format internal-json` fixtures: one per Nix version the
# project's pinned nixpkgs actually provides.
#
# WHY A SET, not one capture. The stream is not a documented interface, and the
# decoder degrades when it meets an event it does not RECOGNISE. What degradation
# cannot cover is a type quietly reused to mean something else: that is
# misreported, not degraded, and nothing in the running tool can detect it. The
# only thing that closes that gap is evidence across versions.
#
# WHY ENUMERATE rather than list. Most `nixVersions.nix_2_*` attribute names in a
# given nixpkgs are removal stubs that throw when evaluated:
#
#   error: nix_2_24 has been removed. use nix_2_31.
#
# A hardcoded list would also stop widening the moment the pin moves, and say
# nothing about it — silent under-coverage, the same class of problem these
# fixtures exist to prevent.
#
# WHY THREE DERIVATIONS. Nix discovers work as it proceeds, so the expected total
# moves (1 -> 2 -> 3 -> 4) and shifts as a running derivation is counted or not. A
# single-derivation capture never exercises that.
#
# WHY A TOKEN. It makes each derivation new, so the capture records a real BUILD
# rather than a cache hit.
#
# A version this machine cannot obtain is SKIPPED and named. Whether a given
# environment can fetch a given Nix is not a property of this project, and a
# coverage gap must be visible rather than fatal or invisible.
set -euo pipefail
cd "$(dirname "$0")"
repo="$(cd ../../.. && pwd)"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
cat > "$work/chain.nix" <<'NIX'
{ token }:
let
  mk = name: deps: extra: derivation {
    name = "nivis-${name}-${token}";
    system = builtins.currentSystem;
    builder = "/bin/sh";
    args = [ "-c" ''
      echo "unpacking sources"
      echo "building '${name}'"
      ${extra}
      echo "installing"
      echo done > $out
    '' ];
    inherit deps;
  };
  a = mk "stage-kernel" [] "echo '  CC  arch/x86/kernel/setup.o'; echo '  LD  vmlinux'";
  b = mk "stage-initrd" [ a ] "echo 'adding module ext4'; echo 'compressing initrd'";
  c = mk "stage-image" [ a b ] "echo 'creating disk image (2048 MiB)'; echo 'copying closure'";
in c
NIX

# --- which versions does the pin actually resolve? -------------------------
echo "==> enumerating the nix versions the pinned nixpkgs resolves"
versions="$(
  nix eval --impure --raw --expr '
    let
      p = (builtins.getFlake "'"$repo"'").inputs.nixpkgs.legacyPackages.${builtins.currentSystem};
      names = builtins.filter (n: builtins.match "nix_2_.*" n != null)
                (builtins.attrNames p.nixVersions);
      ok = builtins.filter (n: (builtins.tryEval (p.nixVersions.${n}.version or null)).success
                               && (p.nixVersions.${n}.version or null) != null) names;
    in builtins.concatStringsSep " " ok
  '
)"
if [ -z "$versions" ]; then
  echo "FAIL: the pinned nixpkgs resolved no nix versions at all" >&2
  exit 1
fi
echo "    $versions"

captured=0
skipped=()

capture() {                      # capture <attr> <nix-store-bin-dir> <version>
  local attr="$1" bin="$2" ver="$3"
  local tok="fixture-$RANDOM$RANDOM"
  local drv
  drv="$("$bin/nix-instantiate" --argstr token "$tok" "$work/chain.nix")"
  {
    echo "# Captured from \`nix-store --realise --log-format internal-json\`."
    echo "# nix version: ${ver}"
    echo "# captured:    $(date -u +%Y-%m-%d)"
    echo "#"
    echo "# Three chained derivations, so the expected total MOVES as Nix"
    echo "# discovers work; a single-derivation capture would leave that untested."
    echo "# Regenerate every fixture with internal/nixlog/testdata/capture.sh."
    "$bin/nix-store" --realise "$drv" --log-format internal-json 2>&1 \
      | grep '^@nix ' | sed "s|$tok|TOKEN|g"
  } > "realise-${attr}.jsonl"
  echo "    ${attr} (nix ${ver}): $(grep -c '^@nix ' "realise-${attr}.jsonl") events"
  captured=$((captured + 1))
}

for attr in $versions; do
  echo "==> ${attr}"
  # --print-out-paths emits every output (out, man, ...), so take the one that
  # actually carries the binaries.
  bin=""
  if paths="$(nix build --no-link --print-out-paths \
                "nixpkgs#nixVersions.${attr}" 2>/dev/null)"; then
    while read -r p; do
      [ -x "$p/bin/nix-store" ] && bin="$p" && break
    done <<< "$paths"
  fi
  if [ -z "$bin" ]; then
    echo "    SKIP: could not obtain ${attr} in this environment"
    skipped+=("$attr")
    continue
  fi
  ver="$("$bin/bin/nix-store" --version | awk '{print $NF}')"
  capture "$attr" "$bin/bin" "$ver"
done

echo
echo "captured ${captured} fixture(s)"
if [ ${#skipped[@]} -gt 0 ]; then
  echo "skipped:  ${skipped[*]}  (not obtainable here — a visible gap, not a failure)"
fi
[ "$captured" -gt 0 ] || { echo "FAIL: no fixtures captured" >&2; exit 1; }
