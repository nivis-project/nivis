# A hermetic tour of the "Road to v1" daily-driver features, against the in-repo
# fake providers (no cloud, no credentials). Exercised by the feature tutorial
# (docs/TUTORIAL-FEATURES.md) and runnable as the flake attr `nivis.tutorial`:
#
#   nivis plan   --attr nivis.tutorial --var env=prod
#   nivis apply  --attr nivis.tutorial --var env=prod
#   nivis output --attr nivis.tutorial --var env=prod
#
# It shows, in one graph:
#   - VARIABLES (mkVars): a typed `env` var (required) + a `replicas` default,
#   - a DATASOURCE (alpha_lookup): read existing infra, feed it into a resource,
#   - the ROUND TRIP across phases: a beta record whose `from` is Nix-built from
#     an alpha token's apply-time value,
#   - OUTPUTS: named values surfaced out of the run.
{ nivis }:
ledger:
let
  inherit (nivis)
    mkResource
    mkData
    str
    toIR
    mkVars
    ;

  # VARIABLES: declare typed inputs. `env` is required (no default => error if
  # unset); `replicas` has a default. Set them with --var / --var-file / NIVIS_VAR_*.
  vars = mkVars {
    env = { type = "str"; }; # required
    replicas = {
      type = "int";
      default = 2;
    };
  } (ledger.vars or { });

  # DATASOURCE: read "existing" infrastructure. The fake alpha_lookup returns
  # result = "found:<query>". We query for the env, so prod and dev differ.
  lookup = mkData {
    provider = "alpha";
    type = "alpha_lookup";
    name = "existing";
    config = {
      query = vars.env;
    };
  };

  # A resource whose label embeds the datasource result: data flows IN.
  token = mkResource {
    provider = "alpha";
    type = "alpha_token";
    name = "app";
    config = {
      label = lookup.refAttr "result"; # from the datasource
    };
  };

  # ROUND TRIP: a beta record whose `from` is a Nix-built string over the token's
  # apply-time value -> resolves in a later phase.
  record = mkResource {
    provider = "beta";
    type = "beta_record";
    name = "app";
    config = {
      from = str [
        "env-${vars.env}-"
        (token.refAttr "value")
      ];
    };
  };
in
toIR {
  providers = {
    alpha = {
      source = "./bin/provider-alpha";
      config = { };
    };
    beta = {
      source = "./bin/provider-beta";
      config = { };
    };
  };
  dataSources = [ lookup ];
  resources = [
    token
    record
  ];
  # OUTPUTS: named values surfaced out of the run (read with `nivis output`).
  outputs = {
    env = vars.env; # a plain variable echoed out
    replicas = vars.replicas; # the int default (or override)
    lookupResult = lookup.refAttr "result"; # from the datasource
    endpoint = record.refAttr "endpoint"; # the round-trip value, from beta
  };
  inherit ledger;
}
