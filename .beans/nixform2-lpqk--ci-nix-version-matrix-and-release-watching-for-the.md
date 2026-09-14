---
# nixform2-lpqk
title: 'CI: nix version matrix and release-watching for the internal-json contract'
status: todo
type: task
priority: low
created_at: 2026-09-11T14:15:01Z
updated_at: 2026-09-14T20:51:33Z
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


---

## Scope widened, 2026-09-14

The `compat-tiers`-style **nix version map** moves here from nixform2-flo8.

That bean originally carried the map alongside the re-renderer. Splitting it
off, because a map and the matrix that exercises it are one piece of work:

- A version map's value is EARLY DETECTION — knowing a new nix broke the
  `internal-json` contract before a user does. Detection requires something to
  run the conformance test across versions periodically. That is this bean's
  matrix. A map without it is a table nobody checks.
- Safety does not depend on the map. `render-nix-build-events` ships
  degrade-never-fail: an unknown `type` is ignored and an unparseable stream
  falls back to raw passthrough. A nix upgrade degrades visibly; it does not
  break.

So this bean now covers:

1. The version map itself — the tier table (`unsupported` / `verified` /
   `compatible by design`), reusing the `compat-tiers` vocabulary the repo
   already has for providers.
2. The CI matrix over the `verified` range, running the conformance test.
3. A `regenerate-fixtures` script, so adding a version is one command.
4. A scheduled job that notices a nix release above the map's ceiling and files
   a bean.

`render-nix-build-events` leaves behind what this builds on: the decoder, one
golden fixture from the pinned nix, and the detected-version reporting at
`verbose`.

### Feasibility, checked 2026-09-14

The pinned nixpkgs exposes `nixVersions.nix_2_4` through `nix_2_34`, so a matrix
has versions to draw on without a separate fetch path. Building or substituting
each is the cost, which is why it belongs in CI rather than a local gate.

Note the caveat already in this bean: confirm the runners can actually fetch
them before designing around it.
