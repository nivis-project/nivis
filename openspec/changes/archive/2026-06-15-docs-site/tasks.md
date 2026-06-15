# Tasks: docs-site

## 1. Spec
- [x] 1.1 Write proposal, tasks, branding spec delta (ADDED docs-site requirement)
- [x] 1.2 `openspec validate docs-site` passes

## 2. Scaffold the mdBook site
- [x] 2.1 `docs-site/book.toml` (title, theme dir, additional-css, git/edit links)
- [x] 2.2 `docs-site/src/SUMMARY.md` + thin pages reusing existing docs (`{{#include}}` where practical)
- [x] 2.3 `docs-site/README.md` — how to build (`mdbook build docs-site`)

## 3. Brand theme
- [x] 3.1 `docs-site/theme/custom.css` — colour tokens + Cinzel/Grotesk/Plex stack with system fallback
- [x] 3.2 `docs-site/theme/head.hbs` — og:image/twitter:image → banner.png, favicon, OG title/description
- [x] 3.3 Copy brand assets the site references (`banner.png`, favicon) into `docs-site/`

## 4. Build + verify + close
- [x] 4.1 `mdbook build docs-site` succeeds
- [x] 4.2 Verify generated HTML carries brand CSS, the font stack, and og:image/twitter:image
- [x] 4.3 README "Docs site" build note
- [x] 4.4 `openspec archive docs-site`; fold requirement into branding spec
- [x] 4.5 Close beans-9qgf; commit as Pim Snel; push
