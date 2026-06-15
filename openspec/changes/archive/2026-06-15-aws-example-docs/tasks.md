# Tasks: aws-example-docs

- [x] 1.1 `nix/example/aws.nix`: `{ terraeNivis }: ledger:` declaring provider
      `aws` (source registry.opentofu.org/hashicorp/aws, empty config) + one
      `aws_s3_bucket` (force_destroy, nixform-test tag, bucket omitted -> AWS
      generates a unique name). toIR with the injected ledger.
- [x] 1.2 `flake.nix`: `terraeNivis.aws = import ./nix/example/aws.nix
      { inherit terraeNivis; }`. Evaluates correctly (real source + resource).
- [x] 1.3 VERIFIED the real CLI flow (AWS_PROFILE=…, AWS_REGION=eu-central-1):
      `tn plan/apply --attr terraeNivis.aws` created bucket
      terraform-20260615154022655900000001; `tn state show` showed it;
      `tn destroy --attr terraeNivis.aws` removed it; confirmed gone in AWS, no
      orphan, account swept clean.
- [x] 1.4 README: "Real providers (AWS)" section with exact commands, the
      real-resource warning, the env-credentials note; replaced the stale
      "out of scope/network-gated" disclaimer with an accurate statement.
- [x] 1.5 docs/GETTING-STARTED.md: a "§7 A real provider (AWS)" section + closing
      note corrected.
- [x] 1.6 `go build`/`go test`/`vet` pass (no Go changes); nix tests + IR
      conformance green; flake attr evals.
- [x] 1.7 `openspec validate aws-example-docs` passes; link M2 (nixform2-2vc3).
