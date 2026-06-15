#!/usr/bin/env bash
# Run the terrae-nivis Nix-library tests:
#   1. property eval (nix/tests/properties.nix)
#   2. conformance: toIR output (phase 0) validates against docs/ir-schema.json
#   3. phased resolution: a derived value resolves to concrete once the ledger
#      supplies its inputs.
#   4. module/expansion property eval (nix/tests/modules.nix)
#   5. conformance: an expanded + module-composed IR validates.
# Exits non-zero on any failure. Hermetic: no network, no binary cache.
set -euo pipefail

cd "$(dirname "$0")/.."

echo "== 1. Nix property tests =="
nix eval --impure --json --file nix/tests/properties.nix > /dev/null
echo "   ok"

echo "== 2. toIR conforms to ir-schema.json (phase 0) =="
nix eval --impure --json \
  --expr 'let nf = import ./nix/lib { }; in (import ./nix/example { terraeNivis = nf; }) { phase = 0; outputs = {}; }' \
  2>/dev/null > /tmp/tn-ir-phase0.json
python3 tests/ir-conformance/check.py validate /tmp/tn-ir-phase0.json
rm -f /tmp/tn-ir-phase0.json

echo "== 3. phased resolution (derived -> concrete) =="
got=$(nix eval --impure --raw \
  --expr 'let nf = import ./nix/lib { }; ir = (import ./nix/example { terraeNivis = nf; }) { phase = 2; outputs = { "alpha.alpha_token.A" = { value = "alpha::0"; }; "beta.beta_record.B" = { endpoint = "beta://rec-alpha::0"; }; }; }; in (builtins.elemAt ir.resources 2).config.label' \
  2>/dev/null)
want="beta://rec-alpha::0::alpha::0"
if [ "$got" != "$want" ]; then
  echo "   FAIL: C.label = '$got', want '$want'" >&2
  exit 1
fi
echo "   ok: C.label resolved to '$got'"

echo "== 4. module/expansion property tests =="
nix eval --impure --json --file nix/tests/modules.nix > /dev/null
echo "   ok"

echo "== 5. expanded + module-composed IR conforms =="
# A module composing a count-expanded resource set + a consumer, serialized to IR.
nix eval --impure --json --expr '
  let
    nf = import ./nix/lib { };
    mod = { ... }: {
      providers.alpha = { source = "p"; config = {}; };
      resources = nf.mkResources {
        provider = "alpha"; type = "alpha_token"; name = "web"; count = 2;
        config = i: { label = "web-${toString i}"; };
      };
    };
  in nf.toModuleIR { modules = [ mod ]; }
' 2>/dev/null > /tmp/tn-ir-modules.json
python3 tests/ir-conformance/check.py validate /tmp/tn-ir-modules.json
rm -f /tmp/tn-ir-modules.json

echo
echo "All Nix tests passed."
