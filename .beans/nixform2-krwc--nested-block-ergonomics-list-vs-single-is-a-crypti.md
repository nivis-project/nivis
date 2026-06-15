---
# nixform2-krwc
title: 'Nested-block ergonomics: list-vs-single is a cryptic apply-time trap'
status: todo
type: feature
priority: normal
tags:
    - discovered
created_at: 2026-06-15T21:22:35Z
updated_at: 2026-06-15T21:22:35Z
---

AWS list-nested blocks must be written as a one-element list in Nix config ([ {...} ]); a bare attrset fails only at apply with a cryptic codec error ('expected array for tftypes.List[...], got map'). Hit twice now: aws_s3_bucket default_tags (beans-5ifi) and aws_ebs_snapshot_import disk_container/user_bucket (during the EC2 tutorial rx5h). Two gaps: (1) tn-gen/nivis gen emits only FLAT attributes — nested blocks aren't in the generated constructor, so users hand-write them and guess list-vs-single; (2) no Nix-side validation against the schema, so the mistake surfaces only at apply against a real provider. Possible fixes: emit nested-block structure (with correct list/single nesting) in the generated constructors; and/or a clearer error from the executor that says 'X is a list-nested block; wrap it in [ ... ]'. Discovered 2026-06-15.
