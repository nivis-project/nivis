# Tasks: changelog-gate

## 1. The gate
- [x] 1.1 `tests/check-changelog.sh`: for each archived change under
      `openspec/changes/archive/<date>-<id>/proposal.md`:
      - exempt by date prefix (archived on/before the cutoff 2026-06-18);
      - else require a `Changelog:` line;
      - if the value is not `none`, require a fingerprint of the entry text to
        appear in `CHANGELOG.md`'s `## [Unreleased]` section.
      Fail naming the change + the convention. Pure read; hermetic.
- [x] 1.2 Chain it into `tests/check-docs-ssot.sh`.

## 2. Convention adoption
- [x] 2.1 This change's `proposal.md` carries a `Changelog:` line (it does:
      `none` — a process gate).
- [x] 2.2 `docs/RELEASING.md`: a short note on the `Changelog:` line + the gate.

## 3. Gate run
- [x] 3.1 `bash tests/check-changelog.sh` green (cutoff exempts the existing
      archive; this change is `none`).
- [x] 3.2 `bash tests/check-docs-ssot.sh` green.
- [x] 3.3 `openspec validate changelog-gate --strict`; archive; close beans-gyea.
      (Archiving this change is itself the first post-cutoff archive the gate
      sees; confirm it passes because the proposal is `Changelog: none`.)
