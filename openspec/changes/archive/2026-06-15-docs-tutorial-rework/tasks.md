# Tasks: docs-tutorial-rework

## 1. Spec
- [x] 1.1 Write proposal, tasks, e2e spec delta (MODIFIED: the AWS tutorial is from-scratch via a fresh flake; ADDED: install doc)
- [x] 1.2 `openspec validate docs-tutorial-rework` passes

## 2. Install doc
- [x] 2.1 `docs/INSTALL.md`: nix run / nix shell / nix profile install / go install — runtime needs noted

## 3. Rewrite the tutorial
- [x] 3.1 `docs/TUTORIAL-AWS-S3.md`: install (link INSTALL) → `nix flake init` → boilerplate flake.nix consuming github:…terrae-nivis exposing terraeNivis.plan → add S3 (explained) → bare tn plan/apply/state/destroy → troubleshooting

## 4. Verify + wire + close
- [x] 4.1 In a scratch dir: `nix flake init` + the documented flake.nix + `tn plan` runs (apply/destroy half already proven live)
- [x] 4.2 `docs-site/src/INSTALL.md` ({{#include}}) + SUMMARY nav; tutorial links to it
- [x] 4.3 Update docs-ssot canonical table + `tests/check-docs-ssot.sh` (INSTALL is canonical)
- [x] 4.4 `mdbook build docs-site` succeeds; `tests/check-docs-ssot.sh` passes
- [x] 4.5 `openspec archive docs-tutorial-rework`; fold into e2e spec
- [x] 4.6 Close beans-807d; commit as Pim Snel; push
