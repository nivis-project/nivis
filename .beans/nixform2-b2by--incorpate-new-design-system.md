---
# nixform2-b2by
title: incorpate new design system
status: completed
type: epic
priority: normal
created_at: 2026-06-15T14:48:01Z
updated_at: 2026-06-15T15:29:43Z
---

Use ~/Downloads/Terrae\ Nivis\ branding\ project.zip

    Read design_handoff_terrae_nivis_brand/README.md and roll the Terrae Nivis brand into this repo. Use the SVGs in assets/ as-is. Specifically: (1) add favicon.svg + .ico + apple-touch-icon.png from the glyph; (2) generate the 1280×640 README hero banner PNG per the spec and put it at the top of README.md; (3) set it as the docs og:image; (4) apply the colour + type tokens to the docs theme; (5) recreate the branded CLI splash in cmd/ with the ANSI colours given. Rename the product to "Terrae Nivis" / tagline "Infrastructure as Nix Code" across README and docs, but ask me before renaming the nixform binary. Work through the acceptance checklist at the bottom of the README.

The README is written to be self-sufficient — it has exact hex values, fonts, the banner/CLI specs, and logo usage rules, so Claude Code can implement it without this conversation.



## Proposed (OpenSpec change: brand-rollout, validated)
Read the handoff (~/Downloads/Terrae Nivis branding project.zip ->
design_handoff_terrae_nivis_brand/README.md). Scoped: vendor logo SVGs (done),
generate favicon/ico/apple-touch-icon from the glyph, README hero banner
(1280x640), docs/BRAND.md tokens, branded CLI splash (ANSI, NO_COLOR-aware),
name+tagline. Raster tooling (rsvg-convert/inkscape/magick) confirmed available.
DEFERRED: docs-site og:image+theme (no site exists -> bean), binary rename
tn->nivis (ask first -> bean).



## Done
OpenSpec brand-rollout (archived 2026-06-15-brand-rollout). Implemented: logo SVGs
in assets/; favicon.svg/.ico + apple-touch-icon.png from the glyph; README hero
banner.png (1280x640, real Cinzel + IBM Plex Mono); docs/BRAND.md tokens; branded
tn CLI splash (ANSI, NO_COLOR/TTY-aware, --version) + tests; README name+tagline+
banner. Binary kept as 'tn' (beans-ijon decision). Deferred: docs-site og:image+
theme (beans-9qgf, no site exists). All gates green.
