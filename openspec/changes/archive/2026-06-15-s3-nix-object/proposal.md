# Proposal: s3-nix-object

## Why
The AWS S3 tutorial creates a bucket — which proves "Terraform providers driven
from Nix," but not the thing this project exists for: **mixing the Nix and
Terraform domains** (beans-yacm). A bucket alone has no Nix-computed content
crossing into a provider resource. Adding a text file whose **content is built by
Nix** — and, better, *derived from the bucket's own apply-time output* — makes the
round trip concrete and visible: Nix → bucket created → the bucket's
AWS-generated name flows back into Nix → becomes the file's content → uploaded as
an S3 object.

## What changes
- **`nix/example/aws.nix`**: add an `aws_s3_object` (`name = "note"`) alongside the
  bucket:
  - `bucket` = a `__ref` to the bucket's output (so the object depends on the
    bucket — a real TF→TF edge),
  - `key = "hello-from-nix.txt"`,
  - `content` = a Nix-built string (`terraeNivis.str [...]`) that **includes the
    bucket's name** (a `__derived` on the bucket output) — so the content cannot
    be known until the bucket exists and Nix re-evaluates. This forces a second
    phase and demonstrates the round trip end to end within one example.
  - `content_type = "text/plain"`.
- **`docs/TUTORIAL-AWS-S3.md`**: extend the config walkthrough to include the
  object, and add a short section — "A file whose content comes from Nix" —
  explaining why this is the point: a value computed in the Nix domain becomes the
  body of a real cloud resource, resolved across phases. Show fetching the object
  from S3 (`aws s3 cp`) so the reader sees the Nix-built content in the real
  world.

## Non-goals
- Uploading a file from disk (`source`) — the demo is *Nix-generated* content, not
  a static asset.
- New Nix-lib capability — `mkResource`/`str`/`refAttr` already produce the
  `__ref`/`__derived` leaves; this is an example + docs change.
- Other object features (encryption, ACLs, versioning).

## Impact
- Changed: `nix/example/aws.nix` (+ one `aws_s3_object`), `docs/TUTORIAL-AWS-S3.md`
  (the object + the "content from Nix" section). The site tutorial page includes
  the doc, so it updates automatically; the docs-SSOT check still applies.
- Verification: applied against real AWS — bucket then object resolve across **≥2
  phases**, the object's content holds the bucket's generated name, the object is
  fetched from S3 to confirm the Nix-built content, then destroyed (no orphan).
- Closes beans-yacm.
