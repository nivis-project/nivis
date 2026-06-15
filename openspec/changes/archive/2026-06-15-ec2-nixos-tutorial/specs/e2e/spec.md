# Spec delta: e2e

## ADDED Requirements

### Requirement: A tested EC2 + NixOS tutorial exists
The docs SHALL include a tutorial (`docs/TUTORIAL-EC2-NIXOS.md`) that launches a
NixOS instance on AWS EC2 with Nivis: a NixOS AMI (built and uploaded with
elastinix, running a minimal HTTP server) is launched via an `aws_instance` and an
`aws_security_group` (ingress on port 80) declared in a Nivis flake, the
instance's `public_ip` is read back into Nix (the round trip), and the running
instance is verified to serve **HTTP 200 on port 80** before being destroyed. A
gated e2e SHALL encode that outcome: launch the instance, poll port 80 until it
returns HTTP 200 (within a timeout), then destroy it leaving no resource behind.

#### Scenario: the instance serves HTTP 200
- GIVEN the EC2+NixOS example (an aws_instance from a NixOS AMI with an HTTP server, plus a security group opening port 80)
- WHEN it is applied and the gated e2e polls the instance's public address
- THEN port 80 returns HTTP 200, and destroy then removes the instance with no orphan.

#### Scenario: the tutorial is discoverable
- WHEN the docs site is built
- THEN an "EC2 + NixOS" tutorial page exists and is reachable from the nav.
