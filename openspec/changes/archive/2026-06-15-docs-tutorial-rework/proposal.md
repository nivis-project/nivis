# Proposal: docs-tutorial-rework

## Why
The AWS S3 tutorial (added in `aws-s3-tutorial`) is not actually "from scratch":
it assumes you're **inside the terrae-nivis repo** — `go build -o bin/tn
./cmd/tn`, the in-repo `nix/example/aws.nix`, and `--attr terraeNivis.aws`. A real
newcomer starts on **their own machine, with an empty directory**, installs `tn`
as a tool, and writes **their own** infra flake that *consumes* terrae nivis as a
dependency. This reworks the tutorial to that reality and factors installation
out into a reusable doc.

Verified the premise before writing: a fresh external flake that takes
`terrae-nivis` as an input, uses its `lib` (`mkResource`/`mkProvider`/`toIR`), and
exposes `terraeNivis.plan` is driven by a bare `tn plan` (the default `--attr`) —
no repo checkout, no `--attr` flag needed.

## What changes
- **Add `docs/INSTALL.md`** — a standalone "install terrae nivis" guide, reusable
  by every tutorial: `nix run github:wearetechnative/terrae-nivis#tn`, an ad-hoc
  `nix shell`, a persistent `nix profile install`, and `go install` / building
  from a clone. It documents what `tn` needs at runtime (Nix on PATH; network for
  the first provider fetch).
- **Rewrite `docs/TUTORIAL-AWS-S3.md`** as genuinely from-scratch, in two parts:
  1. **Install `tn`** — a short step that links to `docs/INSTALL.md`.
  2. **A fresh infra flake** — `mkdir my-infra && cd my-infra && nix flake init`,
     then replace the generated `flake.nix` with boilerplate that:
     - takes `inputs.terrae-nivis.url = "github:wearetechnative/terrae-nivis"`
       (pinned by the generated `flake.lock`),
     - binds `tn = terrae-nivis.lib`,
     - exposes `terraeNivis.plan = ledger: tn.toIR { … }` (the default attribute,
       so plain `tn plan` works),
     - and contains the AWS S3 resource (`mkResource` + `mkProvider` with
       region/tags) explained piece by piece.
     Then: `tn plan` → `tn apply` → `tn state show` → `tn destroy`, run from the
     user's own flake directory, with real output. Plus troubleshooting.
- **Wire into the site + SSOT.** Add an INSTALL site page + nav entry; the
  tutorial's install step links to it. Update the docs-ssot canonical table and
  `tests/check-docs-ssot.sh` to register `docs/INSTALL.md` as the canonical
  install instructions (so they aren't re-pasted into the tutorial/README).

## Non-goals
- EC2/NixOS-image or Hetzner tutorials (beans-rx5h, beans-7g9c).
- Publishing a tagged release of terrae nivis to pin against — the `github:`
  input pinned by `flake.lock` is enough; a tag can come later.
- Changing the `tn` CLI or the flake interface — none needed; the external-flake
  flow already works as shown.

## Impact
- New: `docs/INSTALL.md`, `docs-site/src/INSTALL.md` (+ nav). Rewritten:
  `docs/TUTORIAL-AWS-S3.md`. Changed: docs-ssot table + check.
- Verification: in a scratch directory, `nix flake init` + the documented
  `flake.nix` + `tn plan` is run for real (the apply/destroy half was already
  proven live against AWS in `aws-s3-tutorial`); `mdbook build` succeeds; the
  docs-SSOT check passes.
- Closes beans-807d (supersedes the first cut).
