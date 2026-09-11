---
# nixform2-lpqk
title: 'CI: nix version matrix and release-watching for the internal-json contract'
status: todo
type: task
priority: low
created_at: 2026-09-11T14:15:01Z
updated_at: 2026-09-11T14:15:01Z
blocked_by:
    - nixform2-flo8
---

Follow-up to nixform2-flo8 (re-render nix build events). That change pins the
`--log-format internal-json` contract with golden fixtures and a version map;
this one keeps the pin from going stale.

`nix --log-format internal-json` is not a documented, stability-guaranteed
interface — `type` is a bare integer whose enum depends on `action`, and
`fields` is positional and unnamed (see nixform2-flo8 for the probe). It is
stable in practice only because nix-output-monitor depends on it. Without
automation, the first breakage we hear about is a user's nix upgrade.

## Current state

`.github/workflows/ci.yml` installs a single nix via
`cachix/install-nix-action@v31`. There is no version matrix.

## What to build

1. **A CI matrix** over the supported nix range (the version map's `verified`
   tier), running nixform2-flo8's conformance test on each. A version that
   fails conformance is a red build, not a surprise in the field.

2. **A `regenerate-fixtures` script** so refreshing the golden fixtures for a
   new nix version is one command, not a manual capture. Updating the map
   should be cheap enough that it actually happens.

3. **A scheduled job** that checks for nix releases newer than the version map's
   ceiling and files a bean when one appears. The point is to notice a new nix
   BEFORE a user does.

## Note on environment constraints

CLAUDE.md section 6: the Nix binary cache is not reachable from the dev
environment, and egress is allowlisted. This is CI-side work; confirm the
runners can fetch the nix versions the matrix needs before designing around it.
