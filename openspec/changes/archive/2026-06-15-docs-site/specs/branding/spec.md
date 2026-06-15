# Spec delta: branding

## ADDED Requirements

### Requirement: Branded docs site
The repository SHALL include a buildable static documentation site (mdBook) under
`docs-site/` that reuses the repo's existing Markdown docs and applies the brand
tokens from `docs/BRAND.md`: navy surfaces, ice/silver text, glacier-blue links,
the single Volcanic Ember accent, and the Cinzel / Schibsted Grotesk / IBM Plex
Mono type stack (with a system-font fallback so it renders offline). The built
site SHALL set the 1280×640 banner as the `og:image` and `twitter:image` and
SHALL carry the favicon. The site SHALL build with `mdbook build docs-site`.

#### Scenario: the site builds
- WHEN `mdbook build docs-site` is run
- THEN it succeeds and produces `docs-site/book/index.html`.

#### Scenario: brand theme is applied
- WHEN the built site's HTML/CSS is inspected
- THEN it loads the custom brand CSS and the Cinzel/Schibsted Grotesk/IBM Plex
  Mono font stack, using the brand colour tokens.

#### Scenario: social preview image is set
- WHEN a built page's `<head>` is inspected
- THEN `og:image` and `twitter:image` reference the 1280×640 brand banner.
