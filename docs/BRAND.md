# Nivis: brand reference

Nivis wears the house style of [nivis.tf](https://nivis.tf). That site's
repository, [nivis-project/nivis.tf](https://github.com/nivis-project/nivis.tf),
is the source of the brand: the colour and type tokens are in its
`data/tokens.yaml`, the mark's parameters in its `data/marks.yaml`. This file
records what this repository takes from it, and where each value comes from, so
a change on nivis.tf can be followed here.

## Name and tagline

**Nivis** (Latin, "of snow"). Tagline: **"Terraform providers as first-class
Nix values."** It is the same on nivis.tf, the GitHub organisation, the README,
the docs site and the CLI. Formerly `nixform`, then Terrae Nivis.

## The mark

The mark is a closed curve h(θ) = a + b·cos kθ, drawn as nested, rotated,
translucent copies in an analogous teal to blue family around hue 205. For Nivis
it has three lobes, a lobe ratio of 12, four copies and 20 degrees of rotation
between copies (the `default` entry of `data/marks.yaml`).

| Use                 | Source                                  |
| ------------------- | --------------------------------------- |
| mark                | `https://nivis.tf/favicon.svg`          |
| banner, 1200 by 630 | `https://nivis.tf/social.png`           |
| docs site favicon   | `docs-site/theme/favicon.svg`, a copy   |

Markdown loads both images from nivis.tf, so the repository keeps no copies of
its own. The docs site needs one file on disk for mdBook, and
`tests/check-docs-site.sh` holds it to the SHA-256 of the nivis.tf mark.

## Typography

- **Hind** (400, 500, 600) for text and headings.
- **IBM Plex Mono** (400, 500) for code, the CLI and labels.

Both are on Google Fonts under the OFL.

## Palette

The docs site copies its colours from nivis.tf `data/tokens.yaml`, each under
the token's own name, in `docs-site/theme/custom.css`: `ground`, `surface`,
`ink`, `muted`, `line`, `accent` and `accent-soft`, light and dark.

## CLI colours

Truecolor ANSI, only on a TTY and never when `NO_COLOR` is set. The constants
live in `internal/ui/capability.go`; `nivis` and `nivistutor` both use them.

| Constant | sRGB          | Comes from                              | Used for                    |
| -------- | ------------- | --------------------------------------- | --------------------------- |
| `Accent` | 45, 210, 197  | the mark's first copy, hsl(175 65% 50%) | prompt, spinner             |
| `Name`   | 123, 211, 255 | `tok-func`, oklch(0.83 0.11 235)        | phase headers, commands     |
| `Dim`    | 147, 184, 199 | `code-dim`, oklch(0.76 0.045 225)       | secondary text              |

`Bright` is bold in the terminal's own text colour, for the wordmark. A fixed
light colour would vanish on a light terminal.

The `tok-` and `code-` tokens are nivis.tf's colours for text on its dark code
blocks, which is what a terminal is closest to. The plan-diff colours (green,
yellow, red for create, update, destroy) are not part of the brand: they carry
the meaning every plan output gives them.
