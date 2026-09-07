# The "self-managed state bucket" bootstrap: a config whose state lives in an S3
# backend it is itself responsible for creating. The resources stay the offline
# fake providers, so the sequence is exercised without any cloud.
#
# The backend LOCATION comes from variables so a test (and a reader trying it
# against a local S3-compatible server) can point it somewhere without editing
# this file:
#
#   nivis apply --attr nivis.bootstrapState --backend=local \
#     --var state_bucket=my-bucket --var state_key=app.json --var state_region=eu-west-1
#   nivis state migrate --to-remote --attr nivis.bootstrapState --var ...
#   nivis apply --attr nivis.bootstrapState --var ...
#
# The first apply uses --backend=local because the bucket does not exist yet; the
# migrate moves the document into it; every later run uses the declared backend.
# See docs/REMOTE-STATE.md, "Bootstrapping a self-managed state bucket".
{ nivis }:
ledger:
let
  inherit (nivis)
    mkResource
    toIR
    str
    mkVars
    ;

  vars = mkVars {
    state_bucket = {
      type = "str";
      default = "nivis-bootstrap-state";
    };
    state_key = {
      type = "str";
      default = "bootstrap/app.json";
    };
    state_region = {
      type = "str";
      default = "us-east-1";
    };
    # An S3 endpoint override, for a test server or an S3-compatible store. Empty
    # means "the real S3 endpoint the SDK resolves".
    state_endpoint = {
      type = "str";
      default = "";
    };
  } (ledger.vars or { });

  # Two fake resources with a round trip, so there is real state to move.
  token = mkResource {
    provider = "alpha";
    type = "alpha_token";
    name = "app";
    config = { }; # value/id computed at apply
  };

  record = mkResource {
    provider = "beta";
    type = "beta_record";
    name = "app";
    config = {
      from = str [
        "rec-"
        (token.refAttr "value")
      ];
    };
  };
in
toIR {
  # The backend is static config: variables are resolved during evaluation, so
  # these are plain values by the time the IR is emitted (no refs, no unknowns).
  backend = {
    type = "s3";
    bucket = vars.state_bucket;
    key = vars.state_key;
    region = vars.state_region;
    endpoint = vars.state_endpoint;
  };
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
    token
    record
  ];
  outputs = {
    endpoint = record.refAttr "endpoint";
  };
  inherit ledger;
}
