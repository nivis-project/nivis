# Proposal: aws-s3-tutorial

## Why
`docs/GETTING-STARTED.md` §7 ("A real provider (AWS)") is a terse *reference* —
it assumes you've already done the offline walkthrough, shows the four `tn`
commands, and explains `mkProvider`. It is not a *tutorial*: a newcomer who wants
to create their first real S3 bucket with terrae nivis has no from-scratch,
hand-held path (beans-807d). This adds one, end to end and verified against real
AWS.

## What changes
- **Add `docs/TUTORIAL-AWS-S3.md`** — a from-scratch, step-by-step tutorial:
  prerequisites (Go/Nix, an AWS account), getting `tn` (`nix run`/`go build`),
  configuring AWS credentials, writing the bucket config in Nix **explained line
  by line** (`mkResource` + `mkProvider` with region/tags), `plan` → `apply` →
  inspecting state, then `destroy`, plus a short **troubleshooting** section
  (creds, region, the first-run provider download). Each command shows its real
  output.
- **Respect the single-source-of-truth rule (docs-ssot).** The tutorial is the
  canonical *long-form* AWS walkthrough. Getting-started §7 is **trimmed** to a
  brief intro + a link to the tutorial (it stops repeating the full command set),
  so the `tn apply/state/destroy --attr terraeNivis.aws` walkthrough lives in
  exactly one place. The `nix/example/aws.nix` config remains the one place the
  *example* is defined; the tutorial references it rather than inventing a second
  copy.
- **Add the tutorial to the docs site** — a `docs-site/src/tutorial-aws-s3.md`
  page that `{{#include}}`s `docs/TUTORIAL-AWS-S3.md`, with a `SUMMARY.md` nav
  entry. Update the docs-ssot canonical-owner table and the SSOT check so the new
  canonical block is registered/guarded.

## Non-goals
- EC2 / NixOS-image or Hetzner tutorials — separate beans (beans-rx5h,
  beans-7g9c). This is the S3 tutorial only.
- A new example resource type — reuses `nix/example/aws.nix` (one S3 bucket).
- Teaching AWS itself (IAM, billing) beyond what's needed to run the example;
  links out for credential setup rather than reproducing AWS docs.

## Impact
- New: `docs/TUTORIAL-AWS-S3.md`, `docs-site/src/tutorial-aws-s3.md`, a
  `SUMMARY.md` entry. Changed: getting-started §7 (trimmed to intro + link),
  docs-ssot canonical table + `tests/check-docs-ssot.sh`.
- Verification: the **exact tutorial steps are run against real AWS**
  (`AWS_PROFILE=…`), creating then destroying one S3 bucket, confirming no orphan
  — every command/output in the tutorial is real. `mdbook build docs-site`
  succeeds; the docs-SSOT check passes (no duplicated AWS walkthrough).
- Closes beans-807d.
