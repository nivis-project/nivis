# Tasks: docs-deploy

## 1. Spec
- [x] 1.1 Write proposal, tasks, branding spec delta (deploy requirement)
- [x] 1.2 `openspec validate docs-deploy` passes

## 2. Site URLs for the project subpath
- [x] 2.1 `book.toml`: `site-url = "/terrae-nivis/"`
- [x] 2.2 `head.hbs`: absolute `og:image`/`twitter:image` (github.io project URL)
- [x] 2.3 `mdbook build docs-site` succeeds; verify absolute og:image + subpath asset links in output

## 3. CI workflow
- [x] 3.1 `.github/workflows/docs.yml` — build (install+cache mdbook, build) + deploy (deploy-pages@v4), least-privilege permissions + concurrency
- [x] 3.2 README / docs-site README note the live URL

## 4. Deploy + verify + close
- [x] 4.1 Enable Pages (source = Actions) via gh api
- [x] 4.2 `openspec archive docs-deploy`; fold requirement into branding spec
- [x] 4.3 Commit as Pim Snel; push (triggers the workflow)
- [x] 4.4 Watch the Actions run; confirm the live URL serves (HTTP 200, brand theme, absolute og:image)
- [x] 4.5 Close beans-oeri
