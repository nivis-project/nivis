#!/usr/bin/env bash
# Does the Nix ON THIS MACHINE still produce a log stream the decoder can read?
#
# This is the early-detection half, and it is NOT what the committed fixtures
# do. Those are replayed by `go test ./...` (inside `nix flake check`) and guard
# against a DECODER change breaking an older nix. They can never notice a NEWER
# nix changing the stream, because they are recordings.
#
# So this captures fresh, from whatever nix is installed here, and asserts the
# decoder still extracts the same facts. In CI that nix comes from
# install-nix-action and may well be newer than the flake's pin — which is
# exactly the case worth catching before a user hits it.
#
# It needs a real nix, so it cannot live inside the flake's build sandbox (which
# has none). It runs as its own step, like tests/run-nix-tests.sh.
#
# A machine with no nix SKIPS, reported. Whether a runner has nix is not a
# property of this project.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "== nix log-format conformance (fresh capture) =="

if ! command -v nix-store >/dev/null 2>&1; then
  echo "   skip: no nix-store on PATH (nothing to capture from)"
  exit 0
fi

ver="$(nix-store --version | awk '{print $NF}')"
echo "   nix: ${ver}"

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
  a = mk "stage-kernel" [] "echo '  CC  setup.o'";
  b = mk "stage-initrd" [ a ] "echo 'adding module ext4'";
  c = mk "stage-image" [ a b ] "echo 'creating disk image (2048 MiB)'";
in c
NIX

tok="fresh-$RANDOM$RANDOM"
if ! drv="$(nix-instantiate --argstr token "$tok" "$work/chain.nix" 2>"$work/err")"; then
  echo "   skip: could not instantiate the probe here"
  sed 's/^/        /' "$work/err"
  exit 0
fi

{
  echo "# nix version: ${ver}"
  nix-store --realise "$drv" --log-format internal-json 2>&1 \
    | grep '^@nix ' | sed "s|$tok|TOKEN|g"
} > "$work/fresh.jsonl"

events="$(grep -c '^@nix ' "$work/fresh.jsonl" || true)"
echo "   captured ${events} events"

NIVIS_FRESH_CAPTURE="$work/fresh.jsonl" go test ./internal/nixlog/ -run TestFreshCapture -v 2>&1 \
  | sed -n 's/^    conformance_fresh_test.go:[0-9]*: /   /p;/^--- FAIL/p;/^--- PASS/p'

NIVIS_FRESH_CAPTURE="$work/fresh.jsonl" go test ./internal/nixlog/ -run TestFreshCapture >/dev/null
echo "   ok: nix ${ver} produces a stream the decoder reads"
