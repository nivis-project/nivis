---
# nixform2-lpqk
title: 'CI: nix version matrix and release-watching for the internal-json contract'
status: completed
type: task
priority: low
created_at: 2026-09-11T14:15:01Z
updated_at: 2026-09-14T22:10:44Z
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


---

## Investigation, 2026-09-14 — feasibility and scope, measured

Three things checked before designing anything. Two of them contradict what
this bean (and CLAUDE.md) assumed.

### 1. The Nix binary cache IS reachable from here

`CLAUDE.md` section 6 states "the Nix binary cache are NOT reachable". Measured:

    curl https://cache.nixos.org/nix-cache-info   -> HTTP 200 in 0.04s
    nix build nixpkgs#hello                       -> succeeded

And alternate nix versions substitute in seconds, not minutes:

    nixVersions.nix_2_28  -> OK in 2s
    nixVersions.nix_2_31  -> OK in 4s

So a version matrix is cheap, not expensive. CLAUDE.md section 6 is out of date
on this point, at least in this environment; that is worth correcting separately
rather than designing around.

### 2. The matrix is FOUR versions, not thirty-one

An earlier note in this bean said the pinned nixpkgs exposes `nix_2_4` through
`nix_2_34`. That was wrong: those are attribute NAMES, and most are removal
stubs that throw —

    error: nix_2_24 has been removed. use nix_2_31.

Evaluating each shows what actually resolves:

    nix_2_28 = 2.28.7
    nix_2_30 = 2.30.5+1
    nix_2_31 = 2.31.5
    nix_2_34 = 2.34.7      (the system nix here is 2.34.8)

That is the whole realistic matrix. It also means the "unsupported" tier is
decided by AVAILABILITY, not by our testing: we cannot exercise 2.24 because
nixpkgs will not give it to us.

### 3. The decoder already handles every available version, identically

A three-derivation chain captured under each, replayed through
`internal/nixlog`:

    nix 2.28.7   builds=3  max=3/4  logLines=true
    nix 2.30.5   builds=3  max=3/4  logLines=true
    nix 2.31.5   builds=3  max=3/4  logLines=true
    nix 2.34.8   builds=3  max=3/4  logLines=true

Every version: all three builds named, the derivation count reaching 3 of 4,
and the build's own output lines arriving. The `internal-json` shapes we depend
on — actBuild's fields[0], resBuildLogLine, resProgress on the actBuilds
aggregate — have not moved across 2.28 to 2.34.

### What this does to the design

The three-tier table this bean sketched (`unsupported` / `verified` /
`compatible by design`) is more structure than the evidence supports:

- `verified` is simply "what the pinned nixpkgs offers", all four of which pass
  today.
- `unsupported` is not a judgement we make; it is versions nixpkgs has removed
  and we therefore cannot test.
- `compatible by design` is the only tier doing real work: a nix NEWER than the
  pin, which we degrade for (that behaviour already ships in
  `render-nix-build-events`).

So the map is closer to a two-line statement plus a matrix that keeps it true,
and the `compat-tiers` vocabulary may be borrowed weight rather than a fit. The
valuable parts are the CI matrix, the regenerate script, and the
release-watching job — not the taxonomy.

OpenSpec change: `nix-logformat-conformance`.


---

## Scope settled, 2026-09-14

Two parts of this bean are DROPPED, on the evidence above.

**The tier taxonomy is dropped.** An indexing with one populated category is not
an indexing. All four available versions pass; no version falls in another tier.
`compat-tiers` earns its structure on the provider side because the variation
there is real (schema-extractable or not, e2e-verified or not). For nix versions
there is no such variation, and borrowed vocabulary would lend a precision the
evidence does not support. It would also imply we make a judgement where in fact
nixpkgs decides what we can test at all.

**The separate release-watching job is dropped.** The flake's nixpkgs pin IS the
release watcher, provided the matrix ENUMERATES versions dynamically instead of
hardcoding them:

    names = filter (n: match "nix_2_.*" n != null) (attrNames nixVersions);
    probe = n: if (tryEval nixVersions.${n}.version).success then keep else skip

Bump the pin and new nix versions appear in the matrix on their own. A scheduled
job that files beans is a moving part with its own silent failure mode — nobody
notices a cron job that is itself broken — whereas a matrix that widens with the
pin fails visibly, inside a build that already runs.

**What remains:** fixtures for each available version, a conformance test over
them in CI, and a regenerate script.

Honest about what that buys: TODAY it is a regression guard on the decoder, not
early detection — every available version already passes. It becomes early
warning later, when a pin bump brings a nix whose stream has moved. That is
worth having, and less urgent than this bean originally implied.

Filed separately: `CLAUDE.md` section 6 claims the Nix binary cache is
unreachable, which is false here and actively misleading — it nearly led this
work to design around an obstacle that does not exist.


---

## Summary of Changes

OpenSpec change: `nix-logformat-conformance`. `run-progress` modified (1
requirement); no new capability, no user-facing surface (`Changelog: none`).

### What was built

`internal/nixlog/testdata/capture.sh` now ENUMERATES the nix versions the pinned
nixpkgs resolves (via `tryEval` per attribute, because most `nixVersions.nix_2_*`
names are removal stubs that throw) and captures one fixture per version. Four
were committed: 2.28.7, 2.30.5, 2.31.5, 2.34.8. Each records the version and
capture date in its header.

The replay test DISCOVERS fixtures by glob, so the set widens with the pin, and
asserts the same FACTS from each — every build named, the count advancing, the
build's own output arriving. Measured: all four yield builds=3, count=3/4, and
zero unreadable lines.

### One deviation, and it improved the change

Task 3.1 asked for a CI step running the conformance test. It turned out the
replay test ALREADY runs in CI: `nix flake check` runs `go test ./...`. So a
matrix over committed fixtures would have added nothing — recordings cannot
notice a newer nix.

The gap was the other direction. `tests/check-nix-logformat.sh` now captures
FRESH from whatever nix the runner installed (install-nix-action, quite possibly
newer than the flake's pin) and asserts the decoder still reads it. That is the
early detection this bean wanted:

- committed fixtures  -> a DECODER change breaking an older nix   (regression guard)
- fresh capture       -> a NEWER nix changing the stream          (early detection)

It needs a real nix, so it sits beside `run-nix-tests.sh` rather than inside the
build sandbox, and it SKIPS with a reported reason when there is none —
verified with a stripped PATH.

### Designed against the failure shape this repo keeps hitting

A glob-driven test with no floor passes vacuously on zero inputs, which is
exactly `nixform2-p72l` and `nixform2-pex6`. So the test asserts a minimum
count, verified by deleting the fixtures and watching it fail. And
`TestFixturesCoverDistinctVersions` catches the subtler version: four captures
from the SAME nix would satisfy the count while proving nothing about breadth.

### What this is worth, honestly

Today it is a regression guard — every available version already passes, so the
fixtures catch nothing on the day they land. The fresh-capture step is the part
that can find something new, and only once a runner's nix moves ahead of ours.
Less urgent than this bean originally implied, which is why the tier taxonomy
and the scheduled watcher were dropped rather than built.

Filed separately: `nixform2-irf4` — `CLAUDE.md` section 6 wrongly states the Nix
binary cache is unreachable. That sentence nearly caused this work to design
around an obstacle that does not exist.
