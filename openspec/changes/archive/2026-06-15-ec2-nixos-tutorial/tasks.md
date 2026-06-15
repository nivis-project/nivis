# Tasks: ec2-nixos-tutorial

## 1. Spec
- [x] 1.1 Write proposal, tasks, e2e spec delta (ADDED: tested EC2+NixOS tutorial; instance serves :80 -> 200)
- [x] 1.2 `openspec validate ec2-nixos-tutorial` passes

## 2. Feasibility / AMI
- [x] 2.1 Confirm the AMI path: elastinix builds+uploads a NixOS AMI with a minimal HTTP server; capture how the tutorial references it (and that the build is cache-gated)
- [x] 2.2 Confirm aws_instance + aws_security_group plan through Nivis (codec ok)

## 3. Tutorial
- [x] 3.1 `docs/TUTORIAL-EC2-NIXOS.md`: (1) build+upload AMI via elastinix (HTTP server config); (2) Nivis flake: aws_security_group(:80) + aws_instance(ami, t3.micro), public_ip back into Nix; (3) curl :80 == 200; (4) destroy
- [x] 3.2 Site page + SUMMARY nav; docs-SSOT table/check updated

## 4. Tested outcome (the bean's requirement)
- [x] 4.1 Gated Go e2e: launch the instance via the provider client, poll port 80 until HTTP 200 (timeout), destroy, assert no orphan
- [x] 4.2 Run it live against AWS if the AMI is available; otherwise document the AMI prerequisite + that the e2e takes the AMI id

## 5. Close out
- [x] 5.1 Full gate (build, go test, nix, IR conformance, site, SSOT)
- [x] 5.2 `openspec archive ec2-nixos-tutorial`; fold into e2e spec
- [x] 5.3 Close beans-rx5h; commit as Pim Snel; push
