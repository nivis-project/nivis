---
# nixform2-9qgf
title: Docs site + brand theme + og:image (no static site exists yet)
status: completed
type: feature
priority: low
tags:
    - discovered
created_at: 2026-06-15T15:22:07Z
updated_at: 2026-06-15T16:04:59Z
parent: nixform2-b2by
---

Brand handoff items 3 & 4 (og:image/twitter:image + docs-site theme) have no target: docs/ is plain Markdown, no mdBook/Docusaurus/Astro. Standing up a docs site is a separate decision from the branding rollout. When a site exists, apply docs/BRAND.md tokens (navy surfaces, ice/silver text, glacier-blue links, ember single-accent; Cinzel title, Schibsted Grotesk body, IBM Plex Mono code) and set the 1280x640 banner as og:image/twitter:image. Deferred from brand-rollout.


---
Resolved via OpenSpec change docs-site (archived 2026-06-15-docs-site).
Stood up an mdBook site under docs-site/ that REUSES the repo's Markdown docs via {{#include}} (one source of truth: getting-started, IR contract, testing, design, roadmap, brand) plus bespoke index + real-providers pages. Applied the brand: theme/custom.css maps docs/BRAND.md tokens onto mdBook's navy theme (navy surfaces, ice/silver text, glacier-blue links, single ember accent) with the Cinzel/Schibsted Grotesk/IBM Plex Mono stack from Google Fonts + a system fallback (offline-safe); theme/head.hbs sets og:image + twitter:image to the 1280x640 banner.png and the favicon (handoff items 3 & 4). Build verified: `mdbook build docs-site` succeeds (11 pages), HTML carries the brand CSS, font stack, og:image, and rendered includes. README + docs-site/README document the build. mdBook installed from crates.io (allowlisted); Nix-packaged build remains gated on the binary cache (cf. nixform2-28sn). Hosting/deploy intentionally out of scope.
