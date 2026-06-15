---
# nixform2-fhto
title: update roadmap
status: draft
type: task
priority: normal
created_at: 2026-06-15T18:24:39Z
updated_at: 2026-06-15T19:29:19Z
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
