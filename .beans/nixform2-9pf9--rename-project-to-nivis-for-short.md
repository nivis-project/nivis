---
# nixform2-9pf9
title: rename project to nivis for short
status: completed
type: task
priority: normal
created_at: 2026-06-15T19:12:50Z
updated_at: 2026-06-15T19:43:56Z
parent: nixform2-yh58
---

Terrae Nivis is just too much. The meaning of Nivis a beautiful enough. Belongs to Nix.

Maybe we can change th payoff to "All your base belongs to Nix"

we need a new cli name but this can be nivis as well. Everyone can alias there system as they like


---
DONE via OpenSpec change rename-to-nivis (archived 2026-06-15-rename-to-nivis, epic yh58). Full rename Terrae Nivis -> Nivis: brand + payoff "All your base belongs to Nix"; CLI tn -> nivis with tn-gen folded into `nivis gen` (one binary, cobra subcommand); Go module github.com/wearetechnative/nivis (all imports rewritten); flake default attr nivis.plan, apps/packages `nivis`; docs/CI/scripts/specs swept; asset files renamed nivis-*; docs-site URLs + og:image -> /nivis/; VERSION stays 0.2.0. Verified: build, full go test, nix tests, IR conformance 7/7, mdbook build, docs-SSOT, goreleaser check all pass; `nix run .#nivis` shows the NIVIS splash. NOT pushed — awaiting the GitHub repo rename (terrae-nivis -> nivis); then update origin remote + push. (.pb.go rawDesc go_package left as terrae-nivis on purpose — it's descriptor metadata, not the Go import; blanket-renaming it corrupts the length-prefixed descriptor.)
