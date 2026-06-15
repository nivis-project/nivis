# Spec delta: branding

## ADDED Requirements

### Requirement: README is written Nix-first
The README SHALL address Nix users as its primary audience: its quickstart SHALL
lead with running Nivis via Nix (`nix run …#nivis`) and consuming it as a flake
input (`inputs.nivis` exposing `nivis.plan`), and SHALL present the round-trip
capability as what the tool does (not as a proof-of-concept "definition of
done"), with an honest one-line maturity/status note. Building from source with
Go SHALL appear only as a secondary contributor note, not the entry point. The
README SHALL link to the canonical docs (overview, getting-started, the AWS
tutorial, install) rather than reproducing their command blocks, so the
docs-SSOT check continues to pass.

#### Scenario: the entry point is Nix
- WHEN a reader opens the README
- THEN the first runnable instructions use `nix run`/a flake input (not `go build`), and Go-from-source appears only later as a contributor note.

#### Scenario: no duplicated canonical blocks
- WHEN the docs-SSOT check runs after the rewrite
- THEN it passes: the README links to the canonical walkthroughs/install instead of copying them.
