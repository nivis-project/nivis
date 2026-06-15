# Proposal: readme-nix-first

## Why
The README is written for Go developers evaluating a proof-of-concept: it opens
with `go build ./cmd/...`, frames everything as "the PoC's definition of done,"
and buries the Nix story. But Nivis's audience is **Nix users** managing infra —
they want to `nix run` it and drop it into their own flake, not clone a Go
module. The README should meet them there and present Nivis as a working tool
(beans-tzd8).

## What changes
Rewrite `README.md` for a Nix-first audience:
- **Lead with `nix run` and the flake.** The quickstart is
  `nix run github:wearetechnative/nivis#nivis -- plan/apply` and a fresh-flake
  snippet (`inputs.nivis.url = …; outputs.nivis.plan = ledger: lib.toIR { … };`),
  pointing at the getting-started + AWS tutorials for the full walk. Building from
  source (`go build ./cmd/nivis`) moves to a short **Contributing / from source**
  note near the bottom — present for contributors, not the front door.
- **Keep the round-trip thesis, soften the PoC framing.** The headline stays the
  round trip (provider outputs feed back into Nix across phases — the
  differentiator), reworded as a capability rather than "the PoC's definition of
  done." Add an honest one-line **status** (early, 0.2.x, real providers work).
  Deep "why" stays in `docs/DESIGN.md` / `docs/OVERVIEW.md`.
- **Respect single-source-of-truth.** The README continues to *link* to the
  canonical docs (OVERVIEW, GETTING-STARTED, the AWS tutorial, INSTALL) rather
  than reproducing their command blocks — so the docs-SSOT check still passes
  (README must not contain the canonical fingerprints).

## Non-goals
- Rewriting the docs site or the canonical docs themselves — only the README's
  framing and ordering change; content lives where it already does.
- Removing Go/contributor information — it's demoted, not deleted.
- Changing the brand, tagline, or payoff (already Nivis / "All your base belongs
  to Nix").

## Impact
- Changed: `README.md` (reordered + reframed Nix-first). Possibly a small
  `tests/check-docs-ssot.sh` touch only if a fingerprint's wording moves (the
  canonical blocks themselves don't change, so likely none).
- Verification: `tests/check-docs-ssot.sh` passes (README still links, doesn't
  copy); `mdbook build docs-site` unaffected; the README's Nix commands match the
  real CLI (`nix run .#nivis`, `nivis plan`).
- Closes beans-tzd8.
