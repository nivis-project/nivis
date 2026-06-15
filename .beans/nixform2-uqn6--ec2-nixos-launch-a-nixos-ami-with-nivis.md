---
# nixform2-uqn6
title: 'EC2 + NixOS: launch a NixOS AMI with Nivis'
status: completed
type: epic
priority: normal
created_at: 2026-06-15T21:03:18Z
updated_at: 2026-06-15T22:53:10Z
---

Tutorial + tested e2e: build & upload a NixOS AMI with elastinix (nixpkgs image system; the AMI runs a minimal HTTP server), then drive an aws_instance + security group with Nivis to launch it, and verify the instance serves HTTP 200 on port 80, then destroy. aws_instance already introspects cleanly through Nivis's codec. AMI build/upload is cache-gated (elastinix handles it); the Nivis-driven launch + HTTP check is the focus. Driven by beans-rx5h. OpenSpec: ec2-nixos-tutorial.


---
Update (2026-06-16): rx5h DONE — the EC2+NixOS tutorial is written, wired into the site, and PROVEN LIVE end to end (build NixOS image in Nix -> Nivis upload/import/register/launch -> instance serves HTTP 200 -> destroyed clean, no orphans). Remaining open child: qcwb (have nivis realise Nix store paths in resource config before apply, so a single `nivis apply` builds+ships instead of needing `nix build .#ec2-image` first). Epic stays open for qcwb.


---
EPIC COMPLETE (2026-06-16). Both children done: rx5h (EC2+NixOS tutorial — build a NixOS AMI in Nix, Nivis uploads/imports/registers/launches, instance serves HTTP 200, all proven live) and qcwb (nivis auto-realises __build leaves so a SINGLE `nivis apply` builds the image and ships it — no `nix build` first). The full domain mix works end to end: one Nix expression builds the OS and declares the cloud pipeline; one command applies it.
