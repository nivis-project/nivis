# Spec delta: branding

## ADDED Requirements

### Requirement: Docs site is deployed to GitHub Pages
The repository SHALL deploy the `docs-site/` mdBook site to GitHub Pages as a
project page via a GitHub Actions workflow (Pages source = Actions), triggered on
push to `main`. The workflow SHALL build the site with `mdbook build docs-site`
and publish `docs-site/book` as the Pages artifact, using least-privilege
permissions (`pages: write`, `id-token: write`). The deployed site's social
preview image (`og:image`/`twitter:image`) SHALL be an **absolute** URL so social
crawlers can fetch it, and the site SHALL be configured for its project subpath.

#### Scenario: a push to main deploys the site
- WHEN a commit lands on `main`
- THEN the docs workflow builds the mdBook site and deploys `docs-site/book` to
  GitHub Pages, and the published page returns HTTP 200.

#### Scenario: the social image is an absolute URL
- WHEN a deployed page's `<head>` is inspected
- THEN `og:image` and `twitter:image` are absolute `https://…/banner.png` URLs
  (not site-relative paths).
