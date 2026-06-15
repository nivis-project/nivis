# Proposal: docs-site

## Why
The brand rollout deferred handoff items 3 & 4 — the **og:image/twitter:image**
and the **docs-site theme** — because there was no site to apply them to
(beans-9qgf): `docs/` is plain Markdown with no static-site generator. The brand
exists in the repo (tokens, banner, icons) but has no web surface. This change
stands up a minimal docs site and applies the brand to it, closing the two
deferred items.

## What changes
- Add an **mdBook** site under `docs-site/` (mdBook chosen: a single static
  binary, Markdown-native so it reuses the existing docs, no npm/node toolchain,
  installable from crates.io which is allowlisted here):
  - `docs-site/book.toml` — site config (title "Terrae Nivis", git/edit links,
    additional CSS/JS, custom theme dir).
  - `docs-site/src/SUMMARY.md` + a small set of pages that **reuse the existing
    docs** (intro, getting-started, IR contract, testing, design, brand) rather
    than duplicating them — pages are thin and `{{#include}}` the canonical files
    where practical so there is one source of truth.
- Apply the brand (`docs/BRAND.md`) to the site via a custom theme:
  - `docs-site/theme/custom.css` — the colour tokens (navy surfaces, ice/silver
    text, glacier-blue links, the single ember accent) and the type stack
    (Cinzel display, Schibsted Grotesk body, IBM Plex Mono code), loaded from
    Google Fonts with a **system-font fallback** so the site still renders
    offline.
  - `docs-site/theme/head.hbs` — sets `og:image`/`twitter:image` to the 1280×640
    `banner.png`, the favicon, and Open Graph title/description.
  - Copy the brand assets the site references (`banner.png`, favicon) into
    `docs-site/` so the built site is self-contained.
- Document **how to build the site** (a short note in README + a `docs-site/`
  README) — `mdbook build docs-site` → `docs-site/book/`.

## Non-goals
- Hosting/deploy (GitHub Pages workflow, a custom domain) — out of scope; this
  change produces the buildable source, not a deployment. (A deploy workflow can
  be a separate bean if wanted.)
- Replacing the repo's Markdown docs — the site *reuses* them; `docs/*.md` stay
  the canonical source.
- Vendoring fonts — referenced from Google Fonts with a graceful fallback; no
  binary font files are committed.
- Pinning/​packaging mdBook in the flake — building the site needs `mdbook` on
  PATH (installable from crates.io); a Nix-packaged build is gated on the binary
  cache (cf. beans-28sn) and is not attempted here.

## Impact
- New: `docs-site/` (book.toml, src/, theme/, copied assets, a README).
- Changed: README gains a short "Docs site" build note.
- Verification: `mdbook build docs-site` succeeds and the generated HTML carries
  the brand CSS, the Cinzel/Grotesk/Plex font stack, and the
  `og:image`/`twitter:image` meta pointing at the banner.
- Closes beans-9qgf (brand handoff items 3 & 4).
