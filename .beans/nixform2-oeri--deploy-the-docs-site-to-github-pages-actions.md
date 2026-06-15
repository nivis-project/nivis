---
# nixform2-oeri
title: Deploy the docs site to GitHub Pages (Actions)
status: completed
type: feature
priority: normal
tags:
    - discovered
created_at: 2026-06-15T16:07:24Z
updated_at: 2026-06-15T16:13:05Z
parent: nixform2-b2by
---

Stand up CI deployment of the mdBook docs-site to GitHub Pages as a project page (https://wearetechnative.github.io/terrae-nivis/) via a GitHub Actions workflow (Pages source = Actions). Was explicitly out of scope for the docs-site change (nixform2-9qgf non-goals); requested next. Includes: absolute og:image/site-url for the /terrae-nivis/ subpath, .github/workflows/docs.yml (build mdbook + actions/deploy-pages), enabling Pages, and verifying the live URL serves. OpenSpec: docs-deploy.


---
Done via OpenSpec change docs-deploy (archived 2026-06-15-docs-deploy). Live at https://wearetechnative.github.io/terrae-nivis/ (HTTP 200, navy brand theme, absolute og:image). .github/workflows/docs.yml builds mdbook (installed+cached) and deploys docs-site/book via actions/deploy-pages@v4 on push to main; least-privilege perms + pages concurrency. Project-page subpath handled: book.toml site-url=/terrae-nivis/ (404-page back-links) and head.hbs absolute og:image/twitter:image/og:url. Pages enabled with build_type=workflow via the API. First Actions run (27559722340) succeeded; live URL + banner.png + a deeper page all verified 200. Default github.io domain (no CNAME); Nix-packaged build still gated on the binary cache (nixform2-28sn).
