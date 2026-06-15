# Terrae Nivis docs site

A static documentation site built with [mdBook](https://rust-lang.github.io/mdBook/).
It **reuses** the repository's Markdown docs (via `{{#include}}`) so there is one
source of truth, and applies the [brand tokens](../docs/BRAND.md) through a custom
theme.

**Live:** <https://wearetechnative.github.io/terrae-nivis/> — deployed to GitHub
Pages by `.github/workflows/docs.yml` on every push to `main` (Pages source =
GitHub Actions). It's a project page served under the `/terrae-nivis/` subpath,
which is why `book.toml` sets `site-url = "/terrae-nivis/"`.

## Build

```sh
# mdBook is a single static binary. Install from crates.io if you don't have it:
cargo install mdbook            # or: nix-shell -p mdbook / your package manager

mdbook build docs-site          # -> docs-site/book/
mdbook serve docs-site          # live preview at http://localhost:3000
```

The built site is written to `docs-site/book/` (git-ignored).

## Layout

- `book.toml` — site config (title, theme dir, custom CSS, git/edit links).
- `src/SUMMARY.md` — the nav; `src/*.md` — thin pages that `{{#include}}` the
  canonical docs (`../docs/*.md`, `../DESIGN.md`, `../ROADMAP.md`).
- `theme/custom.css` — the brand palette + Cinzel/Schibsted Grotesk/IBM Plex Mono
  type stack (Google Fonts with a system fallback).
- `theme/head.hbs` — `og:image`/`twitter:image` (the 1280×640 banner) + favicon.
- `banner.png`, `terrae-nivis-emblem.svg`, `favicon.svg` — brand assets copied
  from `../docs/assets` / `../assets` so the built site is self-contained.

## Editing content

Edit the canonical docs (`docs/*.md`, `DESIGN.md`, `ROADMAP.md`) — the site picks
up the change. Pages unique to the site (`index.md`, `real-providers.md`) live in
`src/`.
