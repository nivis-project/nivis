---
# nixform2-fhto
title: update roadmap
status: completed
type: task
priority: normal
created_at: 2026-06-15T18:24:39Z
updated_at: 2026-06-16T13:39:24Z
---

We have realized the PoC milestone of this project. We proofed several concepts:

- usability of tf providers with nix-tf-bridge
- mixing domains
- cyclic dependancies
- etc...

Let's say we have a reached an experimental version of this project and we should now look at the future. 

What funcionality should we need to be useful for a broader audience. From nix
developers and early adapters to enterprice users as the NixOS movement is
gaining traction in enterprice already.

Compared to TerraForm, OpenTofu, Pulumi and CDK what features do we need and how shall we plot this on our roadmap.

My personal thoughts: 

- remote state using s3
- adapted documentation (terraform docs to Terrea Nivis)
- auto complete
- colored output of plan/apply/destroy
- datasources?
- vars and overrule vars


---
## Summary of Changes
DONE. Rewrote docs/ROADMAP.md from a PoC-only plan into a forward-looking, audience-maturity roadmap and built the matching beans structure.

Approach (confirmed with the user): organise by audience maturity (Phase A daily-driver for Nix devs -> B team-ready -> C enterprise); headline DoD for the NEXT milestone = "daily-driver for real projects, no Terraform fallback". Benchmarked the gaps honestly against TF/OpenTofu/Pulumi/CDK (aligned with docs/COMPARISON.md): local-only state, no vars/overrides, no datasources, thin DX, no enterprise controls.

ROADMAP.md now: declares the PoC done + alpha (0.3.x); a "where we are weak" section; the architecture invariants that bind every phase (DESIGN.md); Phase A/B/C with per-theme detail and beans IDs; and the delivered PoC preserved as a History section. Ground-truth checked against the code (Store interface exists but local-only; ReadDataSource unused; no --var; splash exists but no diff coloring).

The user's seed ideas all placed: remote S3 state -> B1; adapted/Terraform-to-Nivis docs -> A5; autocomplete -> A4; colored output -> A3; datasources -> A2; vars and overrule vars -> A1.

Beans created (all todo, tagged `roadmap`, linked from the doc):
- Milestone nixform2-zdj0 "Road to v1" (Phase A) + epics kym5/6e6i/yqd3/oycy/n2rg/z8e1.
- Milestone nixform2-kovh (Phase B) + epics izhk/0oqk/tyzs/cdfj.
- Milestone nixform2-1okn (Phase C) + epics alr9/84fs/m83a/q7fx/7evo.

Each epic is scoped so its tasks become OpenSpec changes; IR-affecting epics (A1 vars-entry, A2 datasource node) note the IR-CONTRACT.md change is a hard gate first.
