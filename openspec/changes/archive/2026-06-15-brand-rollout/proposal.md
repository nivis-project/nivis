# Proposal: brand-rollout

## Why
The project is now **Terrae Nivis** (Latin, "lands of snow"), tagline
**"Infrastructure as Nix Code."** A finished brand identity (logo, colour, type,
CLI/banner specs) was handed off in `~/Downloads/Terrae Nivis branding project.zip`
(`design_handoff_terrae_nivis_brand/README.md`, high-fidelity, with an acceptance
checklist). This change rolls that brand into the repo — icons, README hero
banner, a branded CLI splash, and the name/tagline — using the production SVG
assets as-is. Tracks beans-b2by.

## What changes
- **Vendor the production logo SVGs** into `assets/` (already added):
  `terrae-nivis-emblem.svg` (full emblem, ≥40px) and `terrae-nivis-glyph.svg`
  (single-peak mark, 16–64px / favicons). Used verbatim — the handoff says the
  vector artwork is final; never recolour/stretch/rotate.
- **Icons** (generated from the glyph with the available raster tooling —
  rsvg-convert/inkscape/magick are present): `assets/favicon.svg` (the glyph),
  `favicon.ico` (32px), `apple-touch-icon.png` (180px glyph on the `#0E3157`
  navy tile, ~14% padding).
- **README hero banner** (1280×640 PNG at `docs/assets/banner.png`): the
  `09 · README & social card` artwork — radial navy gradient
  (`#1A4170→#102E52→#08213C`), centred emblem + "TERRAE NIVIS" wordmark (Cinzel
  600) + tagline (`#AECFE6`) + a mono pill. Built as an SVG to the exact token
  spec, then rasterised. Referenced at the top of `README.md`.
- **CLI splash** in `cmd/tn` (Go/ANSI): the `10 · Command line` ASCII peak +
  wordmark + tagline, with the ember accent (`\e[38;2;242;99;46m` for the `❯`
  prompt and "fixpoint reached") and ice-blue (`#AECFE6`) for resource
  names/values. Shown on `tn` with no args / `tn --version`; **respects
  NO_COLOR** and non-TTY (plain output when piped).
- **Name & tagline**: README/docs already say "terrae nivis"; add the formal
  product name **"Terrae Nivis"** + tagline, the banner, and a one-line "formerly
  nixform" note. A **brand tokens reference** at `docs/BRAND.md` (the colour/type
  tokens) so future work has the palette in-repo.

## Non-goals
- **Docs-site og:image + theme** (handoff items 3 & 4): the repo has **no static
  docs site** (`docs/` is plain Markdown — no mdBook/Docusaurus/Astro). There is
  no target to wire `og:image`/`twitter:image` or a theme into. Deferred and
  tracked as its own bean; standing up a docs site is a separate decision, not
  part of a branding rollout. The brand tokens (`docs/BRAND.md`) are recorded so
  that work is ready when a site exists.
- **Renaming the `tn` binary to `nivis`** — both the handoff and beans-b2by say
  *ask the maintainer first*. Out of scope here; flagged for a decision.
- Self-hosting fonts / shipping the HTML brand guide — the guide is a design
  reference, not product (handoff says so explicitly).

## Impact
- New: `assets/` (logo SVGs + generated icons), `docs/assets/banner.png` (+ its
  source SVG), `docs/BRAND.md`, a `cmd/tn` splash; `README.md` gains the banner +
  product name/tagline. No Go logic, IR, or provider behaviour changes — the
  executor/test surface is untouched, so all existing gates must stay green.
- Generated PNG/ICO are committed (reproducible from the committed SVG sources
  via documented commands).
