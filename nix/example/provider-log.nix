# A config over the LOG-EMITTING fake provider (cmd/provider-zeta), for proving
# that provider log output reaches the user as readable notes rather than raw
# provider telemetry. See bean nixform2-ceoh and docs/GETTING-STARTED.md
# ("Provider notes").
#
#   nivis plan --attr nivis.providerLog                          # one note
#   nivis plan --attr nivis.providerLog --provider-log-level=error   # silence
#   nivis plan --attr nivis.providerLog --provider-log-level=trace   # unabridged
#
# `count` controls how many resources are declared: provider-zeta emits its note
# once per planned resource, so more than one resource exercises the collapse
# ("printed once, remaining occurrences counted").
{ nivis }:
ledger:
let
  inherit (nivis)
    mkResource
    toIR
    mkVars
    ;

  vars = mkVars {
    count = {
      type = "int";
      default = 1;
    };
  } (ledger.vars or { });

  notes = builtins.genList (
    i:
    mkResource {
      provider = "zeta";
      type = "zeta_note";
      name = "n${builtins.toString i}";
      config = {
        label = "note-${builtins.toString i}";
      };
    }
  ) vars.count;
in
toIR {
  providers = {
    zeta = {
      source = "provider-zeta";
      config = { };
    };
  };
  resources = notes;
  inherit ledger;
}
