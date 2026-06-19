# Getting started with Nivis: the headline two-provider round trip, from scratch.
# Two fake providers, a direct ref, and Nix-derived values that force multiple
# phases. The starter's flake.nix exposes this as `nivis.plan`, so you run it with
# plain `nivis` (no --attr/--flake):
#
#   nivis plan
#   nivis apply
#   nivis output
#
# alpha_token.A            (alpha)   -- no inputs; A.value computed at apply
#    name = "rec-" + A.value         -- a __derived value
# beta_record.B  (beta)    from=name -- B.endpoint computed at apply
#    final = B.endpoint + "::" + A.value   -- __derived on BOTH providers
# alpha_token.C  (alpha)   label=final
{ nivis }:
ledger:
let
  inherit (nivis) mkResource toIR str;

  A = mkResource {
    provider = "alpha";
    type = "alpha_token";
    name = "A";
    config = { }; # no inputs; value/id computed at apply
  };

  # name = "rec-" + A.value  -> __derived on A.value (forces a phase)
  B = mkResource {
    provider = "beta";
    type = "beta_record";
    name = "B";
    config = {
      from = str [
        "rec-"
        (A.refAttr "value")
      ];
    };
  };

  # final = B.endpoint + "::" + A.value -> __derived on both (forces another)
  final = str [
    (B.refAttr "endpoint")
    "::"
    (A.refAttr "value")
  ];

  C = mkResource {
    provider = "alpha";
    type = "alpha_token";
    name = "C";
    config = {
      label = final;
    };
  };
in
toIR {
  providers = {
    alpha = {
      source = "provider-alpha";
      config = { };
    };
    beta = {
      source = "provider-beta";
      config = { };
    };
  };
  resources = [
    A
    B
    C
  ];
  nixConsumers = [
    {
      id = "systemConfig";
      value = {
        recordEndpoint = B.refAttr "endpoint"; # from beta
        tokenValue = A.refAttr "value"; # from alpha
        combined = final; # from both
      };
    }
  ];
  # Declared stack outputs (read with `nivis output`): one from a single resource,
  # one composed across BOTH providers (resolves across phases like the round trip).
  outputs = {
    token = A.refAttr "value"; # from alpha
    combined = final; # B.endpoint :: A.value, from both providers
  };
  inherit ledger;
}
