# Spec: branding

## Purpose
The Nivis brand in the repo: logo assets, icons, README hero banner,
branded CLI splash, and the colour/type tokens (docs/BRAND.md). Presentation
only — no executor/IR/provider behaviour. Formerly nixform.
## Requirements
### Requirement: Logo assets are vendored and used unmodified
The repository SHALL include the production logo SVGs (`nivis-emblem.svg`,
`nivis-glyph.svg`) under `assets/`, used as-is. The emblem SHALL NOT be
recoloured, stretched, skewed, rotated, shadowed, or rendered below 40px (the
glyph is used below that).

#### Scenario: emblem and glyph present
- WHEN the repo is inspected
- THEN `assets/nivis-emblem.svg` and `assets/nivis-glyph.svg` exist
  and match the handoff artwork byte-for-byte.

### Requirement: Favicon and app icons derived from the glyph
The repository SHALL provide `assets/favicon.svg` (the glyph), `favicon.ico`
(32px), and `apple-touch-icon.png` (180px glyph on the `#0E3157` navy tile with
~14% padding), generated from the glyph SVG.

#### Scenario: icon set exists at the expected sizes
- WHEN the icons are generated
- THEN favicon.svg, favicon.ico (32×32), and apple-touch-icon.png (180×180) exist.

### Requirement: README hero banner
The repository SHALL include a 1280×640 hero banner (`docs/assets/banner.png`)
matching the handoff `09` artwork (radial navy gradient, centred emblem, "TERRAE
NIVIS" wordmark, tagline, mono pill), referenced at the top of `README.md`.

#### Scenario: banner generated and referenced
- WHEN the README is rendered
- THEN it shows a 1280×640 banner image from `docs/assets/banner.png`.

### Requirement: Branded CLI splash
The `nivis` CLI SHALL print a branded splash (ASCII peak, "NIVIS", tagline) with
the ember accent for the prompt/"fixpoint reached" and ice-blue for resource
names, when run with no arguments. It SHALL emit plain (uncoloured) text when
`NO_COLOR` is set or output is not a TTY. Schema codegen is a subcommand of the
same binary (`nivis gen`), not a separate executable.

#### Scenario: splash on no-args invocation
- WHEN `nivis` is run with no arguments on a TTY
- THEN it prints the branded splash with ANSI colours and the wordmark "NIVIS".

#### Scenario: NO_COLOR / piped output is plain
- WHEN `nivis` runs with `NO_COLOR=1` or its output is piped
- THEN the splash contains no ANSI escape codes.

#### Scenario: codegen is a subcommand
- WHEN `nivis gen --provider <p> --out <dir>` is run
- THEN it generates the typed Nix constructors (the former `tn-gen`), from the one `nivis` binary.

### Requirement: Product name and tagline
README and docs SHALL present the product as **Nivis** with the tagline
**"Infrastructure as Nix Code"** and the payoff line **"All your base belongs to
Nix,"** noting its lineage (formerly `nixform`, then Terrae Nivis). A brand tokens
reference (`docs/BRAND.md`) SHALL record the colour and typography tokens.

#### Scenario: name, tagline, payoff, and tokens present
- WHEN the docs are inspected
- THEN README states the name **Nivis** + tagline + the payoff + the lineage note,
  and `docs/BRAND.md` lists the colour and type tokens.

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

### Requirement: Documentation has a single source of truth per topic
Documentation SHALL keep exactly one canonical location per shared topic (e.g.
the AWS real-provider walkthrough, the "how it works" / round-trip overview, the
build/run commands), recorded in `docs-site/README.md`. Every other appearance
SHALL `{{#include}}` that canonical source (in the mdBook site) or link to it
(in `README.md`), and SHALL NOT restate it verbatim. A docs-integrity check under
`tests/` SHALL fail if a designated canonical block is duplicated verbatim
elsewhere, and the mdBook site SHALL still build with the includes in place.

#### Scenario: a shared topic is not copied
- WHEN the repository's Markdown is inspected for a designated canonical block
  (e.g. the AWS `tn apply/state/destroy --attr nivis.aws` walkthrough)
- THEN that block appears in exactly one canonical file, and other documents
  include it or link to it rather than reproducing it.

#### Scenario: duplication is caught
- WHEN the docs-SSOT check runs and a canonical block has been copied verbatim
  into another document
- THEN the check fails and names the duplicated block and file.

#### Scenario: the site still builds
- WHEN `mdbook build docs-site` is run after the includes/links are in place
- THEN it succeeds and no rendered page has lost its content.

### Requirement: README is written Nix-first
The README SHALL address Nix users as its primary audience: its quickstart SHALL
lead with running Nivis via Nix (`nix run …#nivis`) and consuming it as a flake
input (`inputs.nivis` exposing `nivis.plan`), and SHALL present the round-trip
capability as what the tool does (not as a proof-of-concept "definition of
done"), with an honest one-line maturity/status note. Building from source with
Go SHALL appear only as a secondary contributor note, not the entry point. The
README SHALL link to the canonical docs (overview, getting-started, the AWS
tutorial, install) rather than reproducing their command blocks, so the
docs-SSOT check continues to pass.

#### Scenario: the entry point is Nix
- WHEN a reader opens the README
- THEN the first runnable instructions use `nix run`/a flake input (not `go build`), and Go-from-source appears only later as a contributor note.

#### Scenario: no duplicated canonical blocks
- WHEN the docs-SSOT check runs after the rewrite
- THEN it passes: the README links to the canonical walkthroughs/install instead of copying them.

