---
# nixform2-gmmo
title: 'Possible: a derived leaf fed by a data source is unknown at plan, causing a perpetual diff'
status: todo
type: bug
priority: normal
created_at: 2026-09-10T19:49:12Z
updated_at: 2026-09-10T19:49:12Z
parent: nixform2-kovh
---

**This is an observation, not a confirmed defect.** The construction that produced it has been removed, so it can no longer be reproduced from the reporting repo. Recorded because the reasoning points at real behaviour and the next person to use `derived` will likely hit it.

## What was observed

In `nivis-demos` stack/020_vaultwarden_ec2, an `aws_iam_policy` had its `policy` document built with `nivis.derived`, from a single input: the `account_id` of an `aws_caller_identity` data source.

```nix
policy = derived {
  inputs = [ (caller.refAttr "account_id") ];
  render = resolved: builtins.toJSON { ... Resource = "arn:aws:ssm:...:${head resolved}:parameter/..."; };
};
```

`apply` succeeded and wrote the correct document. Every subsequent `plan`, with no configuration change, then reported:

```
~ aws.aws_iam_policy.ssm_read (aws_iam_policy)
```

`nivis state show` confirmed the stored document was complete and correct, with the account id resolved:

```
policy = {"Statement":[{"Action":["ssm:GetParameter"],"Effect":"Allow","Resource":"arn:aws:ssm:eu-central-1:<account>:parameter/..."}],"Version":"2012-10-17"}
```

So the applied state was right, and the plan still wanted to change it. A perpetual diff.

## Why a normalisation explanation was ruled out

IAM policy documents are the classic false positive here: AWS returns them re-serialised, and providers normally suppress that with a diff-suppression function. Two things argue against it:

- The same domain has a second `aws_iam_policy` (`vmimport`) whose `policy` is a plain `builtins.toJSON` string. It reported **no** diff, from the same provider, in the same run.
- The only difference between the two resources is `derived` versus a plain string.

## The hypothesis

`plan.go` asks the provider (`client.Plan` -> `PlanResourceChange`) and takes `NoOp`/`RequiresReplace` from its answer, so the provider decided there was a change. The likely reason is that it was handed an **unknown** value for `policy`.

A `derived` leaf resolves from the outputs ledger. During `plan`, data sources appear not to be read, so `aws.aws_caller_identity.current.account_id` has no value in the ledger, the derived value stays unresolved, and the provider is given an unknown — which it must plan as a change. On `apply` the data source is read, the value resolves, and the correct document is written. Hence: correct state, permanent diff.

## The question this raises

Should data sources be read during `plan` so that `derived` values depending only on them resolve? A data source is readable without changing anything, which is exactly what a plan is allowed to do. If they are deliberately not read at plan time, then a `derived` value fed only by data sources can never be planned as a no-op, and that is worth documenting as a known limitation rather than left to be discovered.

## What would confirm or refute it

A configuration with two resources differing only in `derived` versus a plain equivalent string, applied and then planned. If the derived one reports an update and the plain one does not, the hypothesis holds. Checking whether `PlanResult.UnknownAfterApply` lists the attribute would show it directly.

## Context and workaround

Found in `nivis-demos` while planning the Vaultwarden EC2 demo after a successful apply. The workaround there was to stop using `derived` at all: the account id was already a required configuration variable, so the data source was unnecessary and the document is now a plain string. That is better regardless, but it means the demo no longer exercises the path.
