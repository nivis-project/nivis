# DOCS-GATE.md: the documentation-coverage gate

Code review catches whether the *code* is right. This gate catches whether the
*docs* kept up. As Nivis grows features, "did the documentation follow?" must be a
checkable step, not something an author remembers. This is the docs equivalent of
"testing is part of done" (`CLAUDE.md` §5).

It is deliberately a **judgment gate**: a regex cannot decide whether a change
needs a whole new document, an extra paragraph, or nothing. So the rubric below is
run by the author (a human or an agent), and a lightweight script
(`tests/check-docs-gate.sh`) enforces only that the judgment was **recorded** on
the change, never silently skipped.

## When the gate runs

At the end of every OpenSpec change, **before `openspec archive`**. It is part of
"done", alongside tests.

## The rubric: decide one of four outcomes

Look at what the change introduces (a new user-facing capability? a new CLI flag?
a new Nix-library function? a behaviour change? a new concept?) and decide:

1. **New document.** A new, self-contained **concept or capability** that a user
   would look for by name and that does not belong inside an existing page.
   Examples: variables, datasources, remote state, policy. A feature that
   introduces a *noun a user reasons about* usually wants its own doc.
   - Action: create `docs/<TOPIC>.md` (canonical), add a thin
     `docs-site/src/<topic>.md` that `{{#include}}`s it, and list it in
     `docs-site/src/SUMMARY.md`. Link it from related docs / README.

2. **New paragraph or section.** The capability extends an existing topic and
   belongs *within* an existing document. Example: a new flag on an existing
   command goes in that command's reference; a new tutorial step goes in the
   tutorial.
   - Action: add the section to the existing canonical doc; the site picks it up
     via its existing include.

3. **Modifications only.** The change alters behaviour the docs already describe
   (a flag's default, an error message, a renamed thing). No new prose, but
   existing prose is now wrong.
   - Action: find and correct every place the old behaviour is documented (search
     the docs, do not guess). The nested-block error change is an example: it
     needed only a troubleshooting-line update.

4. **No docs change.** Internal-only: a refactor, a test, a non-user-visible fix.
   - Action: none, but say so explicitly (and why) so a reviewer knows it was
     considered, not forgotten.

When in doubt between (1) and (2): if the topic is something a user would search
for as a standalone subject, prefer a new document; if it only makes sense as part
of an existing page, prefer a paragraph.

## Recording the decision (what the script checks)

Every OpenSpec change's `proposal.md` MUST contain a line beginning with
**`Docs impact:`** stating the outcome and the target(s), for example:

```
Docs impact: new document, docs/VARIABLES.md (+ docs-site page, SUMMARY entry);
README library list updated.
```
```
Docs impact: modifications only; corrected the list-nested error in the AWS S3
tutorial troubleshooting.
```
```
Docs impact: none; internal refactor, no user-visible surface.
```

`tests/check-docs-gate.sh` fails if a change under `openspec/changes/` (active or
archived) has no `Docs impact:` line. It does **not** judge the content; it only
guarantees the call was made and written down. An archived change that predates
this gate is exempt by an allowlist in the script.

## Why this is not redundant with the SSOT check

`tests/check-docs-ssot.sh` verifies the docs that exist are not duplicated and
the site still builds. This gate verifies the docs that **should** exist do. They
are complementary: one guards structure, the other guards coverage.
