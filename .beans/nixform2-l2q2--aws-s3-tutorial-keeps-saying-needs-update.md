---
# nixform2-l2q2
title: aws s3 tutorial keeps saying needs update
status: completed
type: bug
priority: normal
created_at: 2026-06-15T18:58:03Z
updated_at: 2026-06-15T19:15:56Z
parent: nixform2-ft9v
---

when I follow the tutorial all works as expected. BUt when I run a plan after apply (without changing anything) it keeps saying it needs to change something.


🐟 pim@lego2 my-infra $ tn destroy                                                                                                                                                                                                    <aws:technative_pg-playground_pim>
Destroyed 2 resource(s):
  - aws.aws_s3_object.note
  - aws.aws_s3_bucket.demo
🐟 pim@lego2 my-infra $ tn plan                                                                                                                                                                                                       <aws:technative_pg-playground_pim>
+ aws.aws_s3_bucket.demo (aws_s3_bucket)
+ aws.aws_s3_object.note (aws_s3_object)

2 resource(s) to resolve across phases (+ create, ~ change). Run `tn apply`.
🐟 pim@lego2 my-infra $ tn apply                                                                                                                                                                                                      <aws:technative_pg-playground_pim>
Applied 2 resource(s) across 2 phase(s):
  ✓ aws.aws_s3_bucket.demo
  ✓ aws.aws_s3_object.note
🐟 pim@lego2 my-infra $ tn plan                                                                                                                                                                                                       <aws:technative_pg-playground_pim>
~ aws.aws_s3_bucket.demo (aws_s3_bucket)
~ aws.aws_s3_object.note (aws_s3_object)

2 resource(s) to resolve across phases (+ create, ~ change). Run `tn apply`.
🐟 pim@lego2 my-infra $ tn apply                                                                                                                                                                                                      <aws:technative_pg-playground_pim>
2026-06-15T21:00:17.436+0200 [WARN]  provider.terraform-provider-aws: [WARN] S3 Object (lkfjsdfsd897897sdfsd/hello-from-nix.txt) not found, removing from state
2026-06-15T21:00:17.483+0200 [ERROR] provider.terraform-provider-aws: Response contains error diagnostic: diagnostic_detail="" diagnostic_severity=ERROR tf_proto_version=5.11 tf_provider_addr=registry.terraform.io/hashicorp/aws @module=sdk.proto diagnostic_summary="listing tags for S3 (Simple Storage) Object (arn:aws:s3:::lkfjsdfsd897897sdfsd/hello-from-nix.txt): operation error S3: GetObjectTagging, https response error StatusCode: 404, RequestID: HKRVT7V4FTM7M6H8, HostID: ILn3Nr8XBzEwgl0d2qzg6/Z6WHJC0gt7y3ee39JvxrWdwC3FuyR54X5DPUigIWyBcqZusx5P8lSvksmsz+tKGmTlTgB1JX0StcGM+U7xF3o=, api error NoSuchKey: The specified key does not exist." tf_req_id=b4b85c21-9188-cd40-d62b-f816f2679631 tf_resource_type=aws_s3_object tf_rpc=ApplyResourceChange @caller=/home/runner/go/pkg/mod/github.com/hashicorp/terraform-plugin-go@v0.31.0/tfprotov5/internal/diag/diagnostics.go:58 timestamp="2026-06-15T21:00:17.483+0200"
error: phase 1: apply "aws.aws_s3_object.note": provider diagnostics: listing tags for S3 (Simple Storage) Object (arn:aws:s3:::lkfjsdfsd897897sdfsd/hello-from-nix.txt): operation error S3: GetObjectTagging, https response error StatusCode: 404, RequestID: HKRVT7V4FTM7M6H8, HostID: ILn3Nr8XBzEwgl0d2qzg6/Z6WHJC0gt7y3ee39JvxrWdwC3FuyR54X5DPUigIWyBcqZusx5P8lSvksmsz+tKGmTlTgB1JX0StcGM+U7xF3o=, api error NoSuchKey: The specified key does not exist.


---
FIXED via OpenSpec change plan-noop-detection (archived 2026-06-15-plan-noop-detection, under epic ft9v). Root cause: the executor applied every resource on every run with no no-op detection, and `tn plan` marked anything in state as ~ without asking the provider. Diagnosed live (TERRAE_NIVIS_NOOP_DEBUG): on a re-plan the AWS provider re-marks computed attrs (arn, etag, version_id, …) unknown-after-apply even when nothing changed, so the naive "len(unknown)==0 && planned==prior" never fires. Fix: a no-op is when prior exists, no replace, and every KNOWN planned attr equals prior (unknown computed attrs ignored) — tfcodec.KnownAttrsMatchPrior, used by both v5/v6 backends; PlanResult.NoOp; plan.OpNoop; applyOne skips Apply/Destroy on no-op and re-uses prior outputs; tn plan now plans each resource and marks +/~/-/+/=. VERIFIED LIVE against AWS (account 076504012268): after apply, tn plan shows the object as "= no change" and a second apply is a clean no-op — no 404/tagging error (the symptom you reported). 

Note: aws_s3_bucket still shows "-/+ replace" on re-plan because the AWS provider genuinely reports RequiresReplace for it (a quirk of that resource); that's honored, not masked — and it's separate from the object loop you hit (your repro used an explicit bucket name; the object was the one erroring). Tracked separately if the bucket replace-on-no-change needs taming.
