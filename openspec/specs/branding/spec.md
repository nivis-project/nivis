# Spec: branding

## Purpose
The Terrae Nivis brand in the repo: logo assets, icons, README hero banner,
branded CLI splash, and the colour/type tokens (docs/BRAND.md). Presentation
only — no executor/IR/provider behaviour. Formerly nixform.
## Requirements
### Requirement: Logo assets are vendored and used unmodified
The repository SHALL include the production logo SVGs (`terrae-nivis-emblem.svg`,
`terrae-nivis-glyph.svg`) under `assets/`, used as-is. The emblem SHALL NOT be
recoloured, stretched, skewed, rotated, shadowed, or rendered below 40px (the
glyph is used below that).

#### Scenario: emblem and glyph present
- WHEN the repo is inspected
- THEN `assets/terrae-nivis-emblem.svg` and `assets/terrae-nivis-glyph.svg` exist
  and match the handoff artwork byte-for-byte.

### Requirement: Favicon and app icons derived from the glyph
The repository SHALL provide `assets/favicon.svg` (the glyph), `favicon.ico`
(32px), and `apple-touch-icon.png` (180px glyph on the `#0E3157` navy tile with
~14% padding), generated from the glyph SVG.

#### Scenario: icon set exists at the expected sizes
- WHEN the icons are generated
- THEN favicon.svg, favicon.ico (32×32), and apple-touch-icon.png (180×180) exist.

### Requirement: README hero banner
The repository SHALL include a 1280×640 hero banner (`docs/assets/banner.png`)
matching the handoff `09` artwork (radial navy gradient, centred emblem, "TERRAE
NIVIS" wordmark, tagline, mono pill), referenced at the top of `README.md`.

#### Scenario: banner generated and referenced
- WHEN the README is rendered
- THEN it shows a 1280×640 banner image from `docs/assets/banner.png`.

### Requirement: Branded CLI splash
The `tn` CLI SHALL print a branded splash (ASCII peak, "TERRAE NIVIS", tagline)
with the ember accent for the prompt/“fixpoint reached” and ice-blue for resource
names, when run with no arguments. It SHALL emit plain (uncoloured) text when
`NO_COLOR` is set or output is not a TTY.

#### Scenario: splash on no-args invocation
- WHEN `tn` is run with no arguments on a TTY
- THEN it prints the branded splash with ANSI colours.

#### Scenario: NO_COLOR / piped output is plain
- WHEN `tn` runs with `NO_COLOR=1` or its output is piped
- THEN the splash contains no ANSI escape codes.

### Requirement: Product name and tagline
README and docs SHALL present the product as **Terrae Nivis** with the tagline
**"Infrastructure as Nix Code,"** noting it was formerly `nixform`. A brand
tokens reference (`docs/BRAND.md`) SHALL record the colour and typography tokens.

#### Scenario: name, tagline, and tokens present
- WHEN the docs are inspected
- THEN README states the name + tagline + "formerly nixform", and `docs/BRAND.md`
  lists the colour and type tokens from the handoff.

