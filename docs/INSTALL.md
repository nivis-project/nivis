# Installing Nivis

The Nivis CLI is `nivis` (schema codegen is the `nivis gen` subcommand). It is
distributed as a Nix flake, so you can run it without cloning anything. Pick
whichever fits how you work.

> **Runtime needs.** `nivis` shells out to `nix` to evaluate your configuration, so
> **Nix must be on your `PATH`** (with flakes enabled). The first time you use a
> real provider, `nivis` downloads it from the OpenTofu registry (e.g. the AWS
> provider is ~900&nbsp;MB) and caches it, so that first run needs network and a
> little patience.

## Run it ad hoc (no install)

The quickest way: run straight from the flake; Nix builds it on first use and
caches the result:

```sh
nix run github:wearetechnative/nivis#nivis -- --version
nix run github:wearetechnative/nivis#nivis -- plan      # in your infra dir
```

Everything after `--` is passed to `nivis`; codegen is `nivis -- gen …`.

## A throwaway shell

Drop into a shell with `nivis` on `PATH` for the session, handy while iterating:

```sh
nix shell github:wearetechnative/nivis#nivis
nivis --version
```

## Install it persistently

Add `nivis` to your user profile so it's always available:

```sh
nix profile install github:wearetechnative/nivis#nivis
nivis --version
```

Update later with `nix profile upgrade`, remove with `nix profile remove`.

## From a clone (contributors)

If you've checked out the repository:

```sh
nix run .#nivis -- --version          # from the repo root
# or build a binary:
go build -o bin/nivis ./cmd/nivis
nix build .#nivis                     # -> ./result/bin/nivis
```

## Shell completion

`nivis completion <shell>` prints a completion script for `bash`, `zsh`, `fish`,
or `powershell`. It completes commands and flags, and dynamically completes
resource ids (for `state show`, `state rm`, and `--target`) from your state file.

```sh
# bash: load it for the current shell, or drop it in your completions dir
source <(nivis completion bash)

# zsh: write it where your $fpath looks (then restart the shell)
nivis completion zsh > "${fpath[1]}/_nivis"

# fish
nivis completion fish > ~/.config/fish/completions/nivis.fish
```

Run `nivis completion --help` for per-shell details.

## Working with state

Nivis keeps state in a local JSON file (`--state`, default `nivis.state.json`).
Day-to-day:

```sh
nivis state list                 # ids in state (or "No resources in state.")
nivis state show <id>            # one resource's stored attributes
nivis state rm <id>              # drop a resource from state
```

Move the whole state document around with `pull`/`push` (the same shape a future
remote backend uses):

```sh
nivis state pull > backup.json           # whole state to stdout (or --out)
nivis state pull --out backup.json

nivis state push --in backup.json        # replace state from a file
cat backup.json | nivis state push       # or from stdin
```

`push` **replaces all of state**, so it confirms first and reports the resource
counts. Pass `--force` (or `--yes`) to skip the prompt; `--force` is **required**
when the input is piped (non-interactive), so a scripted push is always explicit.

If a command reports that the state is locked by another `nivis` process, another
run holds the advisory lock. Wait for it to finish; if it crashed and left a stale
`*.lock` file, remove that file by hand. (Nivis no longer hangs on a held lock; it
times out with that message.)

## Pinning

The `github:` reference floats on the default branch. For reproducible infra, pin
it in your own flake's `flake.lock` (the [AWS S3 tutorial](TUTORIAL-AWS-S3.md)
does this: Nivis becomes an input, and `nix flake lock` records the exact
revision). Re-pin deliberately with `nix flake update nivis`.
