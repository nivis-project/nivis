# Proposal: docs-deploy

## Why
The branded mdBook docs site (`docs-site/`) builds, but it has no public home —
deployment was explicitly out of scope for the docs-site change (beans-9qgf). The
site is only useful once it's served. This change deploys it to **GitHub Pages**
as a project page via a **GitHub Actions** workflow (Pages source = Actions), and
fixes the one thing a real deployment requires that a local build does not: an
**absolute** `og:image` URL.

## What changes
- **Absolute social-image + site base path.** A project page is served under the
  `/terrae-nivis/` subpath at `https://wearetechnative.github.io/terrae-nivis/`.
  - `docs-site/book.toml`: set `site-url = "/terrae-nivis/"` so mdBook emits
    correct root-relative asset/link paths under the subpath.
  - `docs-site/theme/head.hbs`: make `og:image`/`twitter:image` **absolute**
    (`https://wearetechnative.github.io/terrae-nivis/banner.png`). Social
    crawlers (Slack, Twitter/X, etc.) cannot fetch a site-relative image — the OG
    image must be a full URL. The favicon stays `{{ path_to_root }}`-relative.
- **CI deploy workflow** `.github/workflows/docs.yml`:
  - Triggers on push to `main` (and `workflow_dispatch`).
  - `build` job: checkout, install `mdbook` (cached), `mdbook build docs-site`,
    `actions/upload-pages-artifact` of `docs-site/book`.
  - `deploy` job: `actions/deploy-pages@v4` to the `github-pages` environment.
  - Least-privilege `permissions` (`contents: read`, `pages: write`,
    `id-token: write`) and a `concurrency` group so deploys don't overlap.
- **Enable Pages** with `build_type: workflow` (via the GitHub API) and add a
  short "Deployed at …" note to the README / `docs-site/README.md`.

## Non-goals
- A custom domain / CNAME — using the default `github.io` project page; a domain
  can be a follow-up.
- Building the site via Nix in CI — the workflow installs the `mdbook` binary
  (from crates.io or a setup action); a Nix-packaged build stays gated on the
  binary cache (cf. beans-28sn).
- PR previews / per-branch deploys — only `main` deploys for now.

## Impact
- New: `.github/workflows/docs.yml`. Repo setting: Pages enabled (source =
  Actions). Changed: `docs-site/book.toml` (site-url), `docs-site/theme/head.hbs`
  (absolute og:image), README/docs-site README (live URL).
- Verification: the Actions run succeeds and the live URL
  (`https://wearetechnative.github.io/terrae-nivis/`) serves the site (HTTP 200,
  brand theme, absolute og:image in the page head).
- Closes beans-oeri.
