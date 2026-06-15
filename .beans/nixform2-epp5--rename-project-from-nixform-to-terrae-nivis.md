---
# nixform2-epp5
title: rename project from `nixform` to `terrae nivis`
status: completed
type: epic
priority: normal
created_at: 2026-06-15T14:49:08Z
updated_at: 2026-06-15T15:14:03Z
blocking:
    - nixform2-b2by
---

the cli will be called `tn` the github repo should be terrae-nivis.



## Done
Comprehensive rename nixform -> terrae nivis (CLI tn), one commit. Go module
github.com/wearetechnative/terrae-nivis; cmd/tn + cmd/tn-gen; Nix attr terraeNivis;
env TERRAE_NIVIS_*; proto go_package + generate.sh; ir-schema identity; fake
addrs; default state file; .gitignore; SPDX headers; LICENSE/NOTICE/LICENSING;
README + getting-started; living governance docs + source-of-truth specs.
Preserved: nixform2- beans ID prefix + bean IDs + archived openspec (history).
All gates green; tn CLI smoke-tested. REMAINING (manual, outside repo): rename
the GitHub repo to terrae-nivis and update the origin remote URL.
