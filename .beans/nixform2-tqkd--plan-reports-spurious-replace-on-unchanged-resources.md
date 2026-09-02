---
# nixform2-tqkd
title: plan reports spurious -/+ replace on unchanged resources (stored == desired)
status: completed
type: task
priority: normal
created_at: 2026-08-31T22:04:31Z
updated_at: 2026-09-02T10:15:08Z
parent: nixform2-kovh
---

On an **unchanged, freshly-applied** stack, `nivis plan` reports `-/+` (replace)
for three resources while everything else is `=`:

```
-/+ aws.aws_iam_role.altcha_lambda
-/+ aws.aws_iam_role_policy.altcha_lambda
-/+ aws.aws_lambda_permission.apigw
= aws.aws_amplify_app.site
= aws.aws_lambda_function.altcha
= aws.aws_apigatewayv2_route.challenge      # target is a __derived value
= ...
```

## Evidence it's NOT real drift

`nivis state show` vs the evaluated desired config are identical for the
affected resources, e.g. `aws_lambda_permission.apigw`:

| attr          | stored                                             | desired (eval)                    |
|---------------|----------------------------------------------------|-----------------------------------|
| function_name | technative-prod-website-v2026-altcha               | ref -> lambda.function_name (same)|
| source_arn    | arn:aws:execute-api:eu-central-1:...:3swg.../*/*    | derived from api.execution_arn (same) |
| statement_id  | AllowAPIGatewayInvoke                               | AllowAPIGatewayInvoke             |
| principal     | apigateway.amazonaws.com                            | apigateway.amazonaws.com          |
| action        | lambda:InvokeFunction                              | lambda:InvokeFunction             |

`aws_iam_role.altcha_lambda` likewise: `name`, `path=/`, and `assume_role_policy`
(`{"Statement":[{"Action":"sts:AssumeRole","Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"}}],"Version":"2012-10-17"}`)
all match stored. No attribute actually differs.

## Why it's puzzling (rules out the obvious)

- It's not "resources with `__ref`/`__derived` inputs": `aws_apigatewayv2_route`
  has a `__derived` `target` and correctly plans `=`. So derived/ref values DO
  resolve at plan time in general.
- `aws_iam_role` has a fully **static** config (no refs) and still shows `-/+`.

Common thread among the three: they're all resources with create/delete-only
(all-ForceNew) or quirky read semantics (`aws_lambda_permission`,
`aws_iam_role_policy`), plus `aws_iam_role`. Suggests a plan-side
diff/classification or refresh-read gap — `PlanReport` evaluates once and is
side-effect-free; maybe a read returns partial/empty for these types so plan
treats prior state as absent -> replace.

## Open question / impact

Need to determine whether this reflects real apply behavior:
- If **apply is idempotent** (re-apply reports 0 changes / no replace), this is a
  **plan-accuracy bug**: `plan` over-reports replaces. Misleading but harmless.
- If **apply actually replaces** them each run, it's real churn: the lambda's
  execution role is destroyed+recreated every apply (brief IAM gap).

Repro: apply the stack, then `nivis plan` immediately — the three show `-/+`
with no config change.

## Ask

- Root-cause the classification; `plan` must not report `-/+` for resources whose
  stored state equals desired.
- Confirm/guarantee apply idempotency for all-ForceNew / quirky-read resource
  types (`aws_lambda_permission`, `aws_iam_role_policy`, `aws_iam_role`).


---
DONE. Root cause: Plan sent the raw encoded CONFIG as ProposedNewState
(v5.go/v6.go), skipping Terraform core's objchange.ProposedNew merge. Unset
computed attributes were therefore encoded as UNKNOWN against an existing
resource; SDKv2 providers read that as "this value will change", and when the
attribute is also ForceNew (aws_iam_role.name_prefix,
aws_iam_role_policy.name_prefix, aws_lambda_permission.statement_id_prefix)
they flag requires-replace on every plan — hence the perpetual -/+ on exactly
those types while e.g. aws_iam_role_policy_attachment (no computed+ForceNew
attrs) planned clean. It was NOT plan-display-only: apply consumed the same
result and genuinely destroyed+recreated the three resources every run.

Fix: tfvalue.ProposedMsgPack builds the proposed state per the objchange
contract (config where set; PRIOR value for unset computed attrs; null for
unset plain attrs; unknown only for unresolved refs / creates), wired into both
v5 and v6 Plan. Unit test: internal/tfvalue/proposed_test.go. Verified against
the real AWS provider + live TechNative v2026 stack: plan now reports
"No changes. 14 resource(s) up to date."
