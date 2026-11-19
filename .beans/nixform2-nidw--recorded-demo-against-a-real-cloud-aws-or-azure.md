---
# nixform2-nidw
title: Recorded demo against a real cloud (AWS or Azure)
status: todo
type: task
tags:
    - discovered
created_at: 2026-11-19T19:23:11Z
updated_at: 2026-11-19T19:23:11Z
---

Asked by Pim on 2026-11-19 after the offline demo (nixform2-vydh, assets/demo.gif). The offline demo uses the in-repo fake providers; a second recording should show the round trip against a real provider, AWS or Azure: a plan, an apply across phases, and a value the cloud returned flowing back into Nix (for example an S3 bucket or an EC2 instance whose generated id or address ends up in Nix-generated content). Needs credentials, so it is recorded by hand like the offline one and must not leak account ids, ARNs or keys in the frames. Decide AWS or Azure first; AWS is what nivis supports today.
