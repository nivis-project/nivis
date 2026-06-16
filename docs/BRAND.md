# Nivis: brand reference

**Nivis** (Latin, *"of snow"*) · tagline **"Infrastructure as Nix
Code."** Formerly `nixform`. This file records the brand tokens so future work
(docs site, UI, slides) has the palette and type in-repo, and is the in-repo
source of truth for the logo geometry and treatments.

## Logo

- `assets/nivis-emblem.svg`: full emblem (snow-capped twin summit on a
  navy disc with silver ring + ember star). Use at **≥40px**.
- `assets/nivis-glyph.svg`: simplified single-peak mark for **16-64px**
  (favicons, tabs, avatars).
- Never recolour (beyond the ember star), stretch, skew, rotate, or shadow the
  emblem. Clear space ≥ the ring thickness. Below 40px use the glyph.

## Colour tokens

| Token | Hex | Use |
|---|---|---|
| Ink | `#081726` | deepest background |
| Deep Navy | `#0E3157` | **primary brand**, icon tile, dark surfaces |
| Disc Navy | `#0B2A48` | emblem disc fill |
| Steel Blue | `#2D5E8E` | secondary |
| Glacier Blue | `#4A93C8` | links, cool UI accent |
| Ice Blue | `#AECFE6` | text on navy, tagline |
| Pale Ice | `#DCEDF7` | subtle fills |
| Silver | `#C3D2DE` | ring, hairlines, metallic |
| Snow | `#F5FAFD` | light surfaces, text on navy |
| **Volcanic Ember** | `#F2632E` | **the one warm accent**: the star, key highlights |
| Magma | `#C4361A` | deep warm shade |

## Typography (all Google Fonts, OFL)

- **Cinzel** (600): wordmark / display, Roman caps, letter-spacing ≈ .07em,
  used UPPERCASE ("NIVIS").
- **Schibsted Grotesk** (400/500/600): UI, body, tagline.
- **IBM Plex Mono** (400/500): code, CLI, labels.

## CLI colours (`nivis`)

Truecolor ANSI, applied only on a TTY (honours `NO_COLOR`): ember
`\e[38;2;242;99;46m` for the `❯` prompt and "fixpoint reached"; ice blue
`\e[38;2;174;207;230m` for resource names/values; dim grey for secondary text.

## Regenerating the raster assets

The logo SVGs are the source; the icons and banner PNG are generated from them.
Requires `rsvg-convert` and ImageMagick (`magick`), plus the Cinzel and IBM Plex
Mono fonts available to fontconfig.

```sh
# favicon (32px) + apple-touch-icon (180px glyph on a #0E3157 tile, ~14% padding)
cp assets/nivis-glyph.svg assets/favicon.svg
rsvg-convert -w 32 -h 32 assets/nivis-glyph.svg -o /tmp/f32.png
magick /tmp/f32.png assets/favicon.ico
rsvg-convert -w 180 -h 180 docs/assets/apple-touch-icon.svg -o assets/apple-touch-icon.png

# README hero banner (1280×640): needs Cinzel + IBM Plex Mono on fontconfig
rsvg-convert -w 1280 -h 640 docs/assets/banner.svg -o docs/assets/banner.png
```

The banner/apple-touch source SVGs live in `docs/assets/` / generated inline;
the committed PNGs are reproducible from them.
