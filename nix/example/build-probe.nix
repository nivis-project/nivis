# A config whose resource config carries a `__build` leaf over a derivation that
# has NEVER been built — the case that proves `nivis apply` builds a `__build`
# leaf rather than only substituting it (bean nixform2-ebon).
#
#   nivis apply --attr nivis.buildProbe --var probe_token=<unique>
#
# `probe_token` goes into the derivation's NAME, so a caller can guarantee the
# derivation is in no store and reachable from no substituter. That guarantee is
# the whole point: with a fixed name the first run would build the probe and every
# later run would find it already valid, which is exactly how the original bug
# hid behind a test that appeared to pass.
#
# The probe is a bare `builtins.derivation` with no dependencies — no nixpkgs, in
# keeping with the pure-builtins library — so it builds in milliseconds. Nix
# provides /bin/sh inside the build sandbox.
{ nivis }:
ledger:
let
  inherit (nivis)
    mkResource
    toIR
    mkVars
    drv
    ;

  vars = mkVars {
    probe_token = {
      type = "str";
      default = "fixed";
    };
  } (ledger.vars or { });

  probe = builtins.derivation {
    name = "nivis-build-probe-${vars.probe_token}";
    system = builtins.currentSystem;
    builder = "/bin/sh";
    args = [
      "-c"
      "echo built-by-nivis > $out"
    ];
  };
in
toIR {
  providers = {
    alpha = {
      source = "provider-alpha";
      config = { };
    };
  };
  resources = [
    (mkResource {
      provider = "alpha";
      type = "alpha_token";
      name = "probe";
      config = {
        # The executor must realise this before the provider is handed it.
        label = drv probe;
      };
    })
  ];
  inherit ledger;
}
