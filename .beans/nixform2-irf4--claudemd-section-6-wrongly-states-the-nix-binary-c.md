---
# nixform2-irf4
title: CLAUDE.md section 6 wrongly states the Nix binary cache is unreachable
status: todo
type: bug
priority: normal
tags:
    - discovered
created_at: 2026-09-14T21:45:11Z
updated_at: 2026-09-14T21:45:11Z
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
