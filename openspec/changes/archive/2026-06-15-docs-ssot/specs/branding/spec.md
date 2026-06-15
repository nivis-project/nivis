# Spec delta: branding

## ADDED Requirements

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
  (e.g. the AWS `tn apply/state/destroy --attr terraeNivis.aws` walkthrough)
- THEN that block appears in exactly one canonical file, and other documents
  include it or link to it rather than reproducing it.

#### Scenario: duplication is caught
- WHEN the docs-SSOT check runs and a canonical block has been copied verbatim
  into another document
- THEN the check fails and names the duplicated block and file.

#### Scenario: the site still builds
- WHEN `mdbook build docs-site` is run after the includes/links are in place
- THEN it succeeds and no rendered page has lost its content.
