---
# nixform2-oeri
title: Deploy the docs site to GitHub Pages (Actions)
status: in-progress
type: feature
priority: normal
tags:
    - discovered
created_at: 2026-06-15T16:07:24Z
updated_at: 2026-06-15T16:07:31Z
parent: nixform2-b2by
---

Stand up CI deployment of the mdBook docs-site to GitHub Pages as a project page (https://wearetechnative.github.io/terrae-nivis/) via a GitHub Actions workflow (Pages source = Actions). Was explicitly out of scope for the docs-site change (nixform2-9qgf non-goals); requested next. Includes: absolute og:image/site-url for the /terrae-nivis/ subpath, .github/workflows/docs.yml (build mdbook + actions/deploy-pages), enabling Pages, and verifying the live URL serves. OpenSpec: docs-deploy.
