---
# nixform2-rx5h
title: tutorial for AWS EC2 with nixos as image
status: completed
type: task
priority: normal
created_at: 2026-06-15T16:35:41Z
updated_at: 2026-06-15T22:06:43Z
parent: nixform2-uqn6
---

the outcome of this tutorial should also be tested


---
DONE via OpenSpec change ec2-nixos-tutorial (archived 2026-06-15-ec2-nixos-tutorial). The whole thesis, proven LIVE against AWS (account 076504012268): a NixOS amazon image built in Nix (nginx, return 200) -> Nivis uploads the .vhd -> aws_ebs_snapshot_import -> aws_ami -> aws_security_group(:80) -> aws_instance, 9 resources across 4 phases. Polled the instance: HTTP 200, body "hello from NixOS on EC2, built and launched by Nivis". Destroyed all 9, zero orphans. Single-file domain mix: nix/example/ec2.nix + flake nivis.ec2 BUILD the image (config.system.build.images.amazon) and feed its .vhd store path straight into aws_s3_object.source. Gated Go e2e (internal/plugin/ec2_nixos_aws_test.go) asserts :80==200. Tutorial docs/TUTORIAL-EC2-NIXOS.md + site page + nav + SSOT. Discovered: list-nested blocks (disk_container/user_bucket) need [ {...} ] (krwc); nivis evaluates-not-builds so the image must be `nix build .#ec2-image` before apply (qcwb) — documented.
