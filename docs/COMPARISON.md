<!-- last-verified: 2026-06-16 -->
<!-- This page compares Nivis to other tools using facts about THOSE tools that
     change over time (their versions, features, licenses). It is checked for
     freshness by tests/check-comparison-fresh.sh: update the `last-verified`
     date above whenever you re-confirm the claims against the Sources section
     at the bottom. The script fails the docs gate once the date goes stale. -->

# Nivis vs the usual suspects

How does Nivis relate to the other tools people reach for when they want
infrastructure-as-code, especially in (or near) the Nix world? This page is an
**honest** comparison: where Nivis is genuinely different, and where it is young
and the others are mature.

> **Maturity, stated plainly.** Nivis is **early** (`0.x`, alpha). The round trip
> works across two providers and real providers (AWS today) apply / update /
> replace / destroy, but it has not been run at scale, the surface is small, and
> the contracts (the IR, the flake interface) are the stable parts while the rest
> moves. Everything below should be read with that in mind: a tool is not "better"
> than CloudFormation because a feature table has more checkmarks in its column.
> Maturity, ecosystem, and operational track record are features too, and there
> the established tools lead.

## The one-line positioning

| Tool | One line |
| --- | --- |
| **Nivis** | Terraform/OpenTofu provider resources as **first-class Nix values**, driven by a thin Go executor that spawns **unmodified** provider binaries. Nix is the config; the provider does the work. |
| **OpenTofu** / Terraform | The provider ecosystem and engine. HCL config, its own state, a huge provider registry. Mature, ubiquitous. |
| **Terranix** | Generates **HCL/JSON** from Nix, then hands it to Terraform/OpenTofu to run. Nix as an HCL *generator*. |
| **NixOps 4** | Nix-native deployment orchestrator (the NixOps line, reworked around a resource/provider model). Nix-centric, NixOS-deployment heritage. |
| **Pulumi** | Real programming languages (TS, Python, Go, …) for IaC. Reuses Terraform providers by **compiling** them into per-provider plugins via its bridge. |
| **AWS CDK** | Real languages that **synthesize CloudFormation**. AWS-first; CDKTF variant synthesizes Terraform. |
| **CloudFormation** | AWS's native, declarative, AWS-only IaC service. Managed state, deep AWS integration. |

## What actually makes Nivis different

Three choices, none of which the others combine:

1. **Provider resources are first-class Nix values.** Not generated HCL
   (Terranix), not a separate program (Pulumi/CDK), not a bespoke resource DSL.
   You write `mkResource { … }` and wire outputs with `refAttr` in plain Nix.

2. **Spawn unmodified providers; do not link.** Nivis talks the Terraform plugin
   protocol (`tfprotov5/v6`) over gRPC to the **same provider binaries** OpenTofu
   uses. Contrast Pulumi, which *compiles* each provider's Go into its own plugin
   via a bridge and a maintained SDK fork, per provider. Spawn-not-link is what
   buys universal, zero-per-provider compatibility with the OpenTofu ecosystem,
   at the cost of the tighter integration Pulumi's bridge gives.

3. **The round trip via phased re-evaluation.** A provider-created value (an IP,
   an ID, a generated secret) flows **back into Nix**, which re-evaluates to
   produce dependent config, repeating to a fixpoint. Pulumi gets a live
   `Output<T>` promise model for free because a Pulumi program is a running
   process; Nix is a batch evaluator with no live runtime, so Nivis does the
   honest Nix-shaped thing (re-eval to a fixpoint) rather than pretending to have
   promises. Terranix has no round trip at all: it generates HCL once and stops.

The closest neighbor by *intent* is Terranix (Nix + the Terraform provider
ecosystem); the closest by *mechanism for reusing providers* is Pulumi (both ride
Terraform providers). Nivis sits between them and matches neither: Nix-native
like Terranix, provider-reusing like Pulumi, but generating HCL like neither and
linking providers like neither.

## Feature comparison

Legend: ✅ yes · ⚠️ partial / with caveats · ❌ no · n/a not applicable.

### Essential features

<div class="tn-compact-table" markdown="1">

| Feature | Nivis | OpenTofu/TF | Terranix | NixOps 4 | Pulumi | CDK | CloudFormation |
| --- | :--: | :--: | :--: | :--: | :--: | :--: | :--: |
| Config language | Nix | HCL | Nix → HCL | Nix | TS/Py/Go/… | TS/Py/… | YAML/JSON |
| Reuses Terraform/OpenTofu providers | ✅ (spawn) | ✅ (native) | ✅ (via TF) | ⚠️ | ✅ (bridge) | ⚠️ (CDKTF) | ❌ |
| Multi-cloud / any provider | ✅ | ✅ | ✅ | ⚠️ | ✅ | ⚠️ | ❌ (AWS) |
| Plan / preview before apply | ✅ | ✅ | ✅ (via TF) | ⚠️ | ✅ | ✅ | ✅ (change sets) |
| Apply / update / replace / destroy | ✅ | ✅ | ✅ (via TF) | ✅ | ✅ | ✅ | ✅ |
| State management | ✅ (local) | ✅ | ✅ (TF) | ✅ | ✅ | n/a (CFN) | ✅ (managed) |
| Outputs feed back into config (round trip) | ✅ (phased re-eval) | ⚠️ (HCL refs, no host-lang feedback) | ❌ | ⚠️ | ✅ (`Output<T>`) | ⚠️ | ⚠️ |
| Typed/validated config | ✅ (Nix + schema codegen) | ✅ | ✅ (Nix) | ✅ | ✅ (host lang) | ✅ | ⚠️ |
| Modules / composition | ✅ (Nix modules) | ✅ | ✅ (Nix) | ✅ | ✅ | ✅ | ⚠️ (nested stacks) |
| Mix OS build + cloud in one expr | ✅ (NixOS image → AMI) | ❌ | ❌ | ⚠️ | ❌ | ❌ | ❌ |

</div>

### Enterprise / operational features

This is where Nivis is youngest. Honest status:

<div class="tn-compact-table" markdown="1">

| Feature | Nivis | OpenTofu/TF | Pulumi | CloudFormation |
| --- | :--: | :--: | :--: | :--: |
| Remote / shared state backends | ❌ (local only, today) | ✅ | ✅ (Pulumi Cloud + self-host) | ✅ (managed) |
| State locking | ❌ (today) | ✅ | ✅ | ✅ |
| Drift detection / refresh | ⚠️ (refresh) | ✅ | ✅ | ✅ |
| Policy as code / guardrails | ❌ | ⚠️ (Sentinel/OPA) | ✅ (CrossGuard) | ✅ (Guard/SCP) |
| Secrets handling across the boundary | ✅ (sensitive refs, 0600 ledger) | ✅ | ✅ | ✅ |
| RBAC / teams / audit (hosted) | ❌ | ✅ (TFC/Enterprise) | ✅ (Pulumi Cloud) | ✅ (IAM/CloudTrail) |
| Provider registry / auto-download | ⚠️ (planned; offline by default) | ✅ | ✅ | n/a |
| Production track record / scale | ❌ (alpha) | ✅ | ✅ | ✅ |
| Commercial support | ❌ | ✅ (vendors) | ✅ | ✅ (AWS) |

</div>

### Licensing (a real differentiator)

| Tool | License posture |
| --- | --- |
| **Nivis** | Own code **Apache-2.0**; vendored Terraform-protocol files are **MPL-2.0**. **No BUSL** anywhere. |
| **OpenTofu** | MPL-2.0 (the open fork created after Terraform's BUSL relicense). |
| **Terraform** | **BUSL-1.1** (source-available) since v1.6. |
| **Terranix** | Open source (MIT); generates HCL for whichever engine you run. |
| **Pulumi** | Apache-2.0 core; Pulumi Cloud is a commercial service. |
| **CDK / CloudFormation** | CDK Apache-2.0; CloudFormation is an AWS service. |

## When to pick what

- **You live in Nix and want real, multi-cloud infra with provider outputs feeding
  back into your Nix config** — Nivis is the only tool aimed squarely at that, but
  accept the alpha maturity.
- **You want Nix to author config but run it through battle-tested tooling** —
  Terranix (Nix generates HCL, OpenTofu/Terraform runs it). No round trip, but
  mature and boring in the good way.
- **You want a mature engine and the biggest provider ecosystem, HCL is fine** —
  OpenTofu (open) or Terraform (BUSL).
- **You want general-purpose languages and a hosted control plane** — Pulumi.
- **You are AWS-only and want native, deeply-integrated IaC** — CloudFormation, or
  CDK if you want a real language synthesizing it.
- **You deploy NixOS machines and want a Nix-native orchestrator** — NixOps 4.

## Sources (re-verify against these)

External facts above (versions, licenses, features of *other* tools) drift. When
re-checking, confirm against the upstream docs and update the `last-verified`
date at the top of this file:

- OpenTofu — <https://opentofu.org> · license & registry
- Terraform — <https://developer.hashicorp.com/terraform> · BUSL relicense notes
- Terranix — <https://terranix.org>
- NixOps — <https://github.com/NixOS/nixops>
- Pulumi & the Terraform bridge — <https://www.pulumi.com/docs/> · <https://github.com/pulumi/pulumi-terraform-bridge>
- AWS CDK / CDKTF — <https://docs.aws.amazon.com/cdk/> · <https://developer.hashicorp.com/terraform/cdktf>
- AWS CloudFormation — <https://docs.aws.amazon.com/cloudformation/>

Nivis's own claims are grounded in this repo: `docs/DESIGN.md` (the spawn-not-link
and phased-eval decisions) and `docs/OVERVIEW.md` (the round trip).
