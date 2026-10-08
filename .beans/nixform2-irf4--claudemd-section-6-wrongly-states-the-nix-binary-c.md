---
# nixform2-irf4
title: CLAUDE.md section 6 wrongly states the Nix binary cache is unreachable
status: completed
type: bug
priority: normal
tags:
    - discovered
created_at: 2026-09-14T21:45:11Z
updated_at: 2026-10-08T21:38:30Z
---

Discovered while scoping nixform2-lpqk on 2026-09-14.

## The claim

`CLAUDE.md` section 6, "Environment constraints (read before you hit a wall)":

> Network egress is **restricted to an allowlist** (GitHub, the Go module proxy
> via GitHub, PyPI, npm, crates). The **OpenTofu provider registry and the Nix
> binary cache are NOT reachable.**

## The measurement

    $ curl -sS -o /dev/null -w 'HTTP %{http_code} in %{time_total}s\n' \
        https://cache.nixos.org/nix-cache-info
    HTTP 200 in 0.040251s

    $ nix build --no-link --print-out-paths nixpkgs#hello
    /nix/store/i270m2h1mhfm9fh4iqif6qvaq488lhlv-hello-2.12.3

    $ nix build --no-link --print-out-paths nixpkgs#nixVersions.nix_2_28   # 2s
    $ nix build --no-link --print-out-paths nixpkgs#nixVersions.nix_2_31   # 4s

The cache is reachable and substitution works, in seconds.

## Why this is not harmless

A stated constraint steers every decision that follows it. Scoping
nixform2-lpqk, I first concluded that a multi-version nix matrix could not be
exercised locally and would have to be designed around — purely because
section 6 said the cache was out of reach. The measurement reversed that, and
the resulting change is smaller and better for it.

An instruction file that is wrong about the environment is worse than one that
is silent about it: it produces confident wrong plans rather than curiosity.

## What to check before editing

This was measured in ONE environment (a local NixOS workstation, 2026-09-14).
Do not simply delete the sentence:

- Confirm whether CI runners see the same. The allowlist may differ between a
  developer machine and GitHub Actions, and section 6's wording may have been
  written for a sandbox that genuinely is restricted.
- The `nix flake check` BUILD SANDBOX is a separate question again: tests inside
  it have no nix binary at all, which is why `internal/phase` coverage differs
  there (see scripts/coverage.sh's exception list). That part of section 6's
  spirit is correct even if the cache claim is not.
- The OpenTofu provider registry claim in the same sentence is UNVERIFIED. Do
  not assume it is wrong just because the cache claim is; test it separately.

Likely outcome: narrow the sentence to what is actually true and name where it
applies, rather than a blanket "not reachable".


OpenSpec change: `correct-env-constraints` (tinychange, `skip_specs: true`).
Registry claim now measured too: reachable, and a real provider artifact
downloads end to end. See the change's tasks.md for the probe table.


## Summary of Changes

CLAUDE.md section 6 rewritten. The blanket "the OpenTofu provider registry and
the Nix binary cache are NOT reachable" is gone, replaced by a measurement
table dated 2026-10-06 and an instruction to probe rather than assume, since an
allowlist differs per environment.

Measured, from a developer workstation: `cache.nixos.org` 200,
`registry.opentofu.org` 200, `registry.terraform.io` 200,
`proxy.golang.org` 200, and a `hashicorp/null` 3.2.4 zip downloaded from its
GitHub release at 5,667,683 bytes and unpacked. The registry probe is end to
end (metadata resolved, then the artifact fetched), not a liveness ping. The
bean had flagged the registry half as unverified; it was wrong too.

Two further inaccuracies in the same paragraph were corrected:

- "the Go module proxy **via GitHub**". `GOPROXY` is
  `https://proxy.golang.org,direct`, answering directly.
- "Do not implement provider download from the registry as a blocking step in
  the PoC" was stale, not merely mis-measured. That capability shipped:
  `openspec/specs/registry/spec.md` specifies resolve, download, checksum
  verification and caching, and `openspec/specs/e2e/spec.md` requires
  `tn apply --attr nivis.aws` to drive a real provider with "registry fetch"
  named in the requirement. The section told a reader to avoid building what was
  already built and spec'd.

What the bean asked to preserve, was preserved and made explicit: the
`nix flake check` build sandbox has no `nix` binary, which is why
`tests/run-nix-tests.sh` and `tests/check-nix-logformat.sh` are their own CI
steps and why `scripts/coverage.sh` floors `internal/phase` at 77 (the
sandbox baseline) against 81.4 with `nix` on PATH. The `x-deny-reason` bullet
stays: an allowlist still exists, the correction is about what is on it. The
reason to test against the fake providers is restated as hermeticity, not
reachability, which is why it survives the correction.

CI was checked and is not subject to any of this: a GitHub-hosted runner with
open egress (`.github/workflows/ci.yml`, `cachix/install-nix-action@v31`).

OpenSpec change: `correct-env-constraints` (tinychange, `skip_specs: true`).
Follow-up: nixform2-vy9f, for the four beans that deferred work citing the
false claim.
