#!/usr/bin/env bash
# Regenerate realise-chain.jsonl: a real `--log-format internal-json` stream.
#
# The chain is THREE derivations on purpose. Nix discovers work as it proceeds,
# so the expected total moves (1 -> 2 -> 3 -> 4) and shifts as a running
# derivation is counted or not. A single-derivation capture never exercises
# that, and the decoder's handling of it would go untested.
#
# The token makes the derivation new every run, so the capture records a real
# BUILD rather than a cache hit.
set -euo pipefail
cd "$(dirname "$0")"

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

nixv="$(nix --version | awk '{print $3}')"
tok="fixture-$RANDOM$RANDOM"
drv="$(nix-instantiate --argstr token "$tok" "$work/chain.nix")"

{
  echo "# Captured from \`nix-store --realise --log-format internal-json\` on nix ${nixv}."
  echo "# Three chained derivations, so the expected total MOVES as Nix discovers work;"
  echo "# a single-derivation capture would leave that behaviour untested."
  echo "# Regenerate with internal/nixlog/testdata/capture.sh."
  nix-store --realise "$drv" --log-format internal-json 2>&1 | grep '^@nix ' | sed "s|$tok|TOKEN|g"
} > realise-chain.jsonl

echo "captured $(grep -c '^@nix ' realise-chain.jsonl) events from nix ${nixv}"
