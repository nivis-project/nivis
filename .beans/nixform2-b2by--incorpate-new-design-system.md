---
# nixform2-b2by
title: incorpate new design system
status: todo
type: epic
priority: normal
created_at: 2026-06-15T14:48:01Z
updated_at: 2026-06-15T15:09:10Z
---

Use ~/Downloads/Terrae\ Nivis\ branding\ project.zip

    Read design_handoff_terrae_nivis_brand/README.md and roll the Terrae Nivis brand into this repo. Use the SVGs in assets/ as-is. Specifically: (1) add favicon.svg + .ico + apple-touch-icon.png from the glyph; (2) generate the 1280×640 README hero banner PNG per the spec and put it at the top of README.md; (3) set it as the docs og:image; (4) apply the colour + type tokens to the docs theme; (5) recreate the branded CLI splash in cmd/ with the ANSI colours given. Rename the product to "Terrae Nivis" / tagline "Infrastructure as Nix Code" across README and docs, but ask me before renaming the nixform binary. Work through the acceptance checklist at the bottom of the README.

The README is written to be self-sufficient — it has exact hex values, fonts, the banner/CLI specs, and logo usage rules, so Claude Code can implement it without this conversation.
