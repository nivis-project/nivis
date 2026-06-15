# Installing terrae nivis

The terrae nivis CLI is `tn` (and a codegen helper, `tn-gen`). It is distributed
as a Nix flake, so you can run it without cloning anything. Pick whichever fits
how you work.

> **Runtime needs.** `tn` shells out to `nix` to evaluate your configuration, so
> **Nix must be on your `PATH`** (with flakes enabled). The first time you use a
> real provider, `tn` downloads it from the OpenTofu registry (e.g. the AWS
> provider is ~900&nbsp;MB) and caches it, so that first run needs network and a
> little patience.

## Run it ad hoc (no install)

The quickest way — run straight from the flake; Nix builds it on first use and
caches the result:

```sh
nix run github:wearetechnative/terrae-nivis#tn -- --version
nix run github:wearetechnative/terrae-nivis#tn -- plan      # in your infra dir
```

Everything after `--` is passed to `tn`. The codegen tool is
`…#tn-gen` the same way.

## A throwaway shell

Drop into a shell with `tn` (and `tn-gen`) on `PATH` for the session — handy while
iterating:

```sh
nix shell github:wearetechnative/terrae-nivis#tn github:wearetechnative/terrae-nivis#tn-gen
tn --version
```

## Install it persistently

Add `tn` to your user profile so it's always available:

```sh
nix profile install github:wearetechnative/terrae-nivis#tn
tn --version
```

Update later with `nix profile upgrade`, remove with `nix profile remove`.

## From a clone (contributors)

If you've checked out the repository:

```sh
nix run .#tn -- --version          # from the repo root
# or build a binary:
go build -o bin/tn ./cmd/tn
nix build .#tn                     # -> ./result/bin/tn
```

## Pinning

The `github:` reference floats on the default branch. For reproducible infra, pin
it in your own flake's `flake.lock` (the [AWS S3 tutorial](TUTORIAL-AWS-S3.md)
does this — terrae nivis becomes an input, and `nix flake lock` records the exact
revision). Re-pin deliberately with `nix flake update terrae-nivis`.
