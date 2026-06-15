# Proposal: docs-ssot

## Why
Documentation has drifted into multiple copies of the same content, so a single
edit must be made in several places or the docs disagree (beans-qvx3). Concretely,
today:

- The **AWS "Real providers"** walkthrough exists three times — `README.md`,
  `docs/GETTING-STARTED.md`, and `docs-site/src/real-providers.md` (the site page
  is bespoke prose, not an include).
- The **"How it works" / round-trip** intro exists three times — `README.md`,
  `docs-site/src/index.md`, `docs/GETTING-STARTED.md`.
- The **build/demo command block** (`go build -o bin/tn …` / `nix run .#tn …`)
  exists in both `README.md` and `docs/GETTING-STARTED.md`.

This already bit us: the `nix-provider-config` work had to patch the *same* AWS
caveat in three files, and the docs-deploy work corrected stale "out of scope"
text that had been copied around. The docs-site change established the right
pattern — pages `{{#include}}` the canonical Markdown — but README and the
bespoke site pages still hold parallel copies.

The goal is **one canonical source per topic**, with everything else including or
linking to it, plus a lightweight invariant so new duplication is caught.

## What changes
- **Designate one canonical owner per shared topic** and make every other
  appearance an include or a link, not a copy:
  - *AWS real-provider walkthrough* → canonical in `docs/GETTING-STARTED.md`
    (its "real provider" section). `docs-site/src/real-providers.md` becomes a
    thin `{{#include}}` of that section (via an mdBook anchor/range include or a
    dedicated `docs/` fragment); `README.md` keeps a **short** teaser + a link,
    not the full command set.
  - *"How it works" / round-trip* → canonical prose in **one** `docs/` file
    (`docs/OVERVIEW.md`); `docs-site/src/index.md` includes it and getting-started
    links to it. README keeps only the headline paragraph + a link.
  - README is a **lean teaser**: hero, one-paragraph pitch, a minimal quickstart
    (build + plan/apply), then links into `docs/` and the live site. Because
    GitHub renders plain Markdown and cannot process mdBook `{{#include}}`, README
    **links** to the canonical docs rather than including them; the long
    walkthroughs it used to restate now live once in `docs/`.
  - *Build/run commands* → canonical in `docs/GETTING-STARTED.md`; README shows
    the minimal quickstart and links to the full steps.
- **Extract shared fragments** under `docs/` where a section needs to appear in
  more than one rendered place, so includes have a stable target (mdBook
  `{{#include file:anchor}}` ranges, or small `docs/_fragments/*.md`).
- **Add a docs-integrity check** (`tests/check-docs-ssot.sh` or a small Python
  checker, wired into `tests/`): it fails if a designated canonical block is
  found duplicated verbatim elsewhere, and verifies the site still builds with
  the includes. This makes the SSOT rule enforceable, not just aspirational.
- **Record the convention** in `docs-site/README.md` (and a short note in the
  repo README): "canonical docs live in `docs/` / the file noted per topic; the
  site and README include or link, never copy."

## Non-goals
- Rewriting the *content* of the docs — this is a de-duplication/structure pass,
  not a docs rewrite. Wording changes only where needed to turn a copy into a
  teaser+link.
- Changing the spec→`openspec/specs/` flow — those are already single-source.
- A docs generator beyond mdBook, or moving everything into the site — README and
  in-repo Markdown stay first-class; the site reuses them.

## Impact
- Changed: `README.md` (full duplicated sections become teasers + links),
  `docs-site/src/{index,real-providers}.md` (bespoke prose → includes/links),
  possibly new `docs/_fragments/` (or anchored ranges in existing docs),
  `docs-site/README.md` (the convention).
- New: a docs-SSOT check under `tests/`, runnable locally (and suitable for CI).
- Verification: `mdbook build docs-site` still succeeds with the new includes; the
  SSOT check passes; no rendered page loses content (spot-checked).
- Closes beans-qvx3.
