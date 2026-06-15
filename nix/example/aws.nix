# A minimal REAL-provider example: one AWS S3 bucket via the hashicorp/aws
# provider (resolved + verified + cached from the OpenTofu registry by the
# executor). Drive it with the tn CLI:
#
#   AWS_PROFILE=<profile> tn apply   --attr terraeNivis.aws
#   tn state show aws.aws_s3_bucket.demo
#   AWS_PROFILE=<profile> tn destroy --attr terraeNivis.aws
#
# This CREATES a real (free-tier) S3 bucket and then destroys it.
#
# Provider settings live in Nix: `region` (and any other non-secret settings,
# e.g. default_tags) are declared here via `mkProvider` and flow into the
# provider's Configure call. Only credentials come from the environment (the AWS
# SDK default chain — AWS_PROFILE or AWS_ACCESS_KEY_ID/…), since we never bake
# secrets into Nix. `bucket` is omitted so AWS assigns a globally-unique name;
# force_destroy lets `tn destroy` delete it unconditionally.
{ terraeNivis }:
ledger:
let
  inherit (terraeNivis) mkResource mkProvider toIR;

  bucket = mkResource {
    provider = "aws";
    type = "aws_s3_bucket";
    name = "demo";
    config = {
      force_destroy = true;
      tags = { nixform-test = "terrae-nivis-aws-example"; };
    };
  };
in
toIR {
  providers = {
    aws = mkProvider {
      source = "registry.opentofu.org/hashicorp/aws";
      config = {
        # Region in Nix, not the environment. Change to your region.
        region = "eu-central-1";
        default_tags = {
          tags = { managed-by = "terrae-nivis"; };
        };
      };
    };
  };
  resources = [ bucket ];
  inherit ledger;
}
