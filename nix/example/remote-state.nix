# A tiny config that stores its state in a REAL S3 bucket (the M2 remote-state
# backend), while the resources themselves stay the offline fake providers — so
# the tutorial exercises remote state + locking without creating any real cloud
# resources. See docs/TUTORIAL-REMOTE-STATE.md.
#
#   AWS_PROFILE=<profile> nivis apply   --attr nivis.remoteState
#   AWS_PROFILE=<profile> nivis plan    --attr nivis.remoteState   # reads state from S3
#   AWS_PROFILE=<profile> nivis force-unlock --attr nivis.remoteState
#
# Set the bucket/region to one you own (state is written to s3://<bucket>/<key>).
{ nivis }:
ledger:
let
  inherit (nivis) mkResource toIR str;

  # Two fake resources with a round trip, just so there is real state to store.
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
  # The remote-state backend: state is stored in this S3 object. CHANGE the bucket
  # and region to one you own. Credentials are NOT here (they come from the AWS
  # chain, e.g. AWS_PROFILE). Only the location is config.
  backend = {
    type = "s3";
    bucket = "REPLACE-WITH-YOUR-BUCKET";
    key = "nivis-tutorial/remote-state/app.json";
    region = "eu-west-1";
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
    endpoint = record.refAttr "endpoint"; # the round-trip value
  };
  inherit ledger;
}
