---
# nixform2-7gdt
title: improve console output
status: completed
type: task
priority: normal
created_at: 2026-09-08T14:46:52Z
updated_at: 2026-09-14T10:58:01Z
parent: nixform2-kovh
---

somwething like:

module.instance.aws_security_group.ec2nix_security_group: Creating...
module.ami.aws_iam_role.vmimport_role: Creating...
aws_ebs_volume.data: Creating...
aws_iam_role.vaultwarden: Creating...
module.ami.aws_s3_bucket.ec2nix_bucket: Creating...
aws_s3_bucket.backups: Creating...
module.ami.aws_s3_bucket.ec2nix_bucket: Creation complete after 1s [id=vaultwarden-demo-demo-076504012268]
module.ami.aws_iam_policy.vmimport_policy: Creating...
module.ami.aws_s3_object.image_upload: Creating...
aws_iam_role.vaultwarden: Creation complete after 1s [id=vaultwarden-demo-role]
aws_iam_role_policy_attachment.ssm: Creating...
aws_iam_role_policy_attachment.cloudwatch: Creating...
aws_iam_role_policy.vaultwarden_env: Creating...
aws_iam_instance_profile.vaultwarden: Creating...
module.ami.aws_iam_role.vmimport_role: Creation complete after 1s [id=terraform-20260908144437872100000002]
aws_iam_role_policy_attachment.ssm: Creation complete after 0s [id=vaultwarden-demo-role-20260908144438927200000004]
aws_iam_role_policy_attachment.cloudwatch: Creation complete after 0s [id=vaultwarden-demo-role-20260908144439113300000005]
module.ami.aws_iam_policy.vmimport_policy: Creation complete after 0s [id=arn:aws:iam::076504012268:policy/terraform-20260908144438696800000003]
module.ami.aws_iam_role_policy_attachment.vmpimport_attach: Creating...
aws_iam_role_policy.vaultwarden_env: Creation complete after 0s [id=vaultwarden-demo-role:vaultwarden-demo-env]
aws_s3_bucket.backups: Creation complete after 1s [id=vaultwarden-demo-076504012268-backups]
aws_s3_bucket_public_access_block.backups: Creating...
aws_s3_bucket_versioning.backups: Creating...
aws_iam_role_policy.backups: Creating...
aws_s3_bucket_server_side_encryption_configuration.backups: Creating...
aws_s3_bucket_lifecycle_configuration.backups: Creating...
module.instance.aws_security_group.ec2nix_security_group: Creation complete after 2s [id=sg-0ef35322466156e5b]
module.instance.aws_security_group_rule.ec2nix_security_group_egress_rule: Creating...
aws_s3_bucket_public_access_block.backups: Creation complete after 1s [id=vaultwarden-demo-076504012268-backups]
module.instance.aws_security_group_rule.ec2nix_security_group_ingress_rule["https"]: Creating...
aws_iam_role_policy.backups: Creation complete after 1s [id=vaultwarden-demo-role:vaultwarden-demo-backups]
module.instance.aws_security_group_rule.ec2nix_security_group_ingress_rule["http-acme"]: Creating...
module.ami.aws_iam_role_policy_attachment.vmpimport_attach: Creation complete after 1s [id=terraform-20260908144437872100000002-20260908144439831300000006]
module.instance.aws_security_group_rule.ec2nix_security_group_egress_rule: Creation complete after 0s [id=sgrule-559491905]
aws_s3_bucket_server_side_encryption_configuration.backups: Creation complete after 1s [id=vaultwarden-demo-076504012268-backups]
module.instance.aws_security_group_rule.ec2nix_security_group_ingress_rule["https"]: Creation complete after 1s [id=sgrule-3110033829]
module.instance.aws_security_group_rule.ec2nix_security_group_ingress_rule["http-acme"]: Creation complete after 1s [id=sgrule-74364069]
aws_s3_bucket_versioning.backups: Creation complete after 3s [id=vaultwarden-demo-076504012268-backups]
aws_iam_instance_profile.vaultwarden: Creation complete after 6s [id=vaultwarden-demo-profile]
aws_ebs_volume.data: Still creating... [10s elapsed]
aws_ebs_volume.data: Creation complete after 11s [id=vol-038f6be6b8f78f3c7]
module.ami.aws_s3_object.image_upload: Still creating... [10s elapsed]
aws_s3_bucket_lifecycle_configuration.backups: Still creating... [10s elapsed]
module.ami.aws_s3_object.image_upload: Still creating... [20s elapsed]
aws_s3_bucket_lifecycle_configuration.backups: Still creating... [20s elapsed]
module.ami.aws_s3_object.image_upload: Still creating... [30s elapsed]
aws_s3_bucket_lifecycle_configuration.backups: Still creating... [30s elapsed]
module.ami.aws_s3_object.image_upload: Still creating... [40s elapsed]
aws_s3_bucket_lifecycle_configuration.backups: Still creating... [40s elapsed]
module.ami.aws_s3_object.image_upload: Still creating... [50s elapsed]
aws_s3_bucket_lifecycle_configuration.backups: Still creating... [50s elapsed]
aws_s3_bucket_lifecycle_configuration.backups: Creation complete after 57s [id=vaultwarden-demo-076504012268-backups]
module.ami.aws_s3_object.image_upload: Still creating... [1m0s elapsed]
module.ami.aws_s3_object.image_upload: Still creating... [1m10s elapsed]
module.ami.aws_s3_object.image_upload: Still creating... [1m20s elapsed]
module.ami.aws_s3_object.image_upload: Still creating... [1m30s elapsed]
module.ami.aws_s3_object.image_upload: Still creating... [1m40s elapsed]
module.ami.aws_s3_object.image_upload: Still creating... [1m50s elapsed]


---

## Exploration outcome (2026-09-11)

OpenSpec change: **`stream-run-progress`** (store `nivis`) — proposal written.

Scope: typed progress-event contract in `internal/phase|destroy|refresh`, a
renderer in `cmd/nivis` that owns stderr (plain + live-TTY), incremental
commit-as-completed output, a three-channel model (stdout result / stderr
narrative / stderr ephemeral live region), `--log-level`, and `--color`.

Out of scope, deliberately:
- `--output=json` — the event model makes it cheap later, but it is a public
  contract deserving its own change.
- Re-rendering Nix's own events — **nixform2-flo8**. Needs a renderer that owns
  the terminal first. Carries its own contract work (degrade-never-fail, golden
  conformance fixtures, a compat-tiers-style nix version map), because
  `--log-format internal-json` is not a stability-guaranteed interface.
- CI matrix + release-watching to keep that version map current — **nixform2-lpqk**.

`stream-run-progress` DOES wire raw Nix passthrough at `verbose`, though: it is a
few lines, it kills the worst symptom immediately, and raw passthrough stays
permanently as the floor beneath any prettier rendering. Raw output and the live
region are mutually exclusive — they fight for the same cursor.

Discovered work filed from this exploration:
- **nixform2-01kd** — the config is evaluated one extra time per run
  (`openStore` discards its graph). Should land BEFORE `stream-run-progress`,
  or the new output narrates the bug.
- **nixform2-6qr9** — phase boundaries measure dependency depth, not Nix round
  trips. Decides whether the phase narrative in the output tells the real story.

Key design note on the original complaint: Terraform's apply output is a flat
interleaved wall because ~10 resources run concurrently, forcing an address
prefix on every line and a `Still creating...` heartbeat per in-flight resource.
Nivis applies one node at a time and has a phase hierarchy, so it does not
inherit either problem — one live status line replaces N heartbeats.


---

## Summary of Changes

OpenSpec change: `stream-run-progress`. New capability `run-progress`; `cli` and
`executor` modified.

**The event contract.** New `internal/progress`: the engines emit FACTS (a
single `Event` struct with a `Kind`) and never presentation text. `Driver.Progress
io.Writer` is gone — it baked sentences into the engine, so verbosity, colour and
alternate renderings had nowhere to live. `progress.Emit` tolerates a nil
observer, so an engine built without one behaves exactly as before.

**Who emits.** `internal/phase` (evaluation, phase boundaries with their node
count, each node with its resolved op and duration, each build),
`internal/destroy`, `internal/refresh`, and `internal/plugin` (provider spawn,
emitted BEFORE resolving the source, because resolving is the slow half).

**The renderer.** New `internal/ui`: `plain` (one line per transition) and `live`
(committed scrollback above, an ephemeral region below, repainted on a ticker).
It is the sole owner of stderr. Three channels: stdout = the result, unchanged
and byte-identical regardless of terminal or verbosity; stderr = the narrative;
stderr-on-a-terminal = the live region, never in a pipe. The state-lock messages
moved from stdout to the narrative.

**Flags.** `--log-level` (`quiet|info|verbose|debug`, or `NIVIS_LOG`, flag wins)
and `--color` (`auto|always|never`, with `CLICOLOR_FORCE` beside `NO_COLOR`).
Cursor capability is probed SEPARATELY from colour, so `--color=never` leaves the
progress display on and forcing colour into a pipe does not turn it on.

**Nix output.** `nix eval` and `nix-store --realise` stderr is teed rather than
discarded, reaching the user at `verbose`. Raw passthrough and the live region
are mutually exclusive by construction — the level that enables one disables the
other — and a test asserts no level enables both.

### Three findings worth recording

1. **Drift could not use `reflect.DeepEqual`.** A provider read decodes the
   resource at its FULL schema, so a converged resource returns attributes the
   stored state never had, as nil. `DeepEqual` called that drift, which would
   have marked every resource in a stack as drifted and made the whole
   "3 of 40 drifted" summary a lie. Fixed with `state.Drifted`, found by a test
   on a deliberately converged fixture.
2. **The live renderer repainted per event** in its first form, defeating the
   ticker — 501 repaints for 500 events. Only a committed line now repaints
   synchronously.
3. **`providerlog` needed no change at all.** The renderer hands out a managed
   writer, so the sink keeps its shape and its whole existing suite passes
   untouched — a stronger guarantee than rewriting it.

### Deviations from the plan

- Task 2.5's refresh discriminator is an explicit `Refresh` field on the event,
  not inferred. The first attempt used a heuristic and was fragile.
- Task 4.1 required no `providerlog` source change (see above).

Coverage: `internal/ui` 91.3%, `internal/progress` 100%, `internal/phase` 81.6%,
total 70.6%. Docs: new `docs/OUTPUT.md`, on the site and linked from
GETTING-STARTED.

Follow-ups remain open: `nixform2-flo8` (re-render Nix events, now unblocked),
`nixform2-lpqk`, `nixform2-6qr9`, `nixform2-p72l`.
