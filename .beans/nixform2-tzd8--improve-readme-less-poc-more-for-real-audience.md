---
# nixform2-tzd8
title: improve readme, less PoC more for real audience
status: completed
type: task
priority: normal
created_at: 2026-06-15T19:41:14Z
updated_at: 2026-06-15T20:24:23Z
---

---
DONE via OpenSpec change readme-nix-first (archived 2026-06-15-readme-nix-first). README reworked Nix-first: quickstart leads with `nix run github:wearetechnative/nivis#nivis` + a fresh-flake snippet (inputs.nivis -> nivis.plan = ledger: lib.toIR {...}), then `nivis plan/apply/state/destroy`. Round-trip thesis kept (the differentiator) but reworded from "the PoC's definition of done" to a capability + an honest "early but real, 0.2.x" status line. Go/go build demoted to a "Contributing / building from source" section near the bottom. SSOT respected — README links to the canonical docs (OVERVIEW/GETTING-STARTED/AWS tutorial/INSTALL) and contains none of the canonical fingerprints; docs-SSOT check passes, site builds, the embedded flake snippet evaluates (schemaVersion 1).
