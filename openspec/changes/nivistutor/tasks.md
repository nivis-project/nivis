# Tasks: nivistutor

## 1. Tutorials as self-contained starters
- [ ] 1.1 `nix/example/tutorial-features-0.4/`: move the current `tutorial.nix`
      here as the config; add a standalone `flake.nix` (nivis input + `nivis.plan`
      = the config) and a `README.md` (the `nivis plan/apply/output` next steps,
      and the `nix shell …#tutor` line for the fakes).
- [ ] 1.2 `nix/example/tutorial-getting-started/`: a from-scratch starter built on
      the headline two-provider round trip (from `default.nix`): `flake.nix`,
      config, `README.md`.
- [ ] 1.3 Repo flake: point `nivis.tutorial` at the features-0.4 starter's config
      (keep the attr name so the milestone-notes gate + the docs include stay
      valid), and update the milestone-notes generator's tutorial path if needed.
- [ ] 1.4 `tests/run-nix-tests.sh` / conformance: each starter config still
      produces conforming IR; the bare-name `source` is unchanged.

## 2. nivistutor binary
- [ ] 2.1 `cmd/nivistutor`: embed the starter directories with `go:embed`
      (`nix/example/tutorial-*/**`). An interactive flow: welcome, list tutorials,
      pick one, choose new-subdir vs current-dir, write the files, print next
      steps. Do NOT run nivis.
- [ ] 2.2 A non-interactive mode (flags: `--tutorial <name> --dir <path>`) for
      scripting/tests; refuse to overwrite existing files without `--force`.
- [ ] 2.3 cobra command + a branded greeting consistent with `nivis`' splash.

## 3. Flake wiring
- [ ] 3.1 `flake.nix`: a `nivistutor` package (buildGoModule, `cmd/nivistutor`)
      and a `#tutor` app (`apps.tutor.program = .../bin/nivistutor`). The providers
      stay available via `#tutor` (re-export) or the existing `#fake-providers` so
      one `nix shell` carries nivis + tutor + fakes.

## 4. Tests
- [ ] 4.1 `cmd/nivistutor`: non-interactive scaffold writes the expected files
      (flake.nix + config + README) for each tutorial into a temp dir.
- [ ] 4.2 No-clobber: scaffolding over an existing file without `--force` errors,
      state unchanged.
- [ ] 4.3 Each embedded starter has a `flake.nix` (guard against a tutorial added
      without its flake).

## 5. Docs
- [ ] 5.1 The feature tutorial + getting-started: a short "scaffold it with
      nivistutor" path for a sandbox (`nix shell …#nivis …#tutor; nivistutor`).
      No em dashes.
- [ ] 5.2 If the feature-tutorial doc moves/renames for the per-release scheme,
      keep the docs-site include + the release-note markers consistent.

## 6. Gate
- [ ] 6.1 `gofmt`, `go build ./...`, `go test ./...` green.
- [ ] 6.2 `bash tests/run-nix-tests.sh` + `bash tests/check-docs-ssot.sh` green
      (incl. milestone-notes + changelog gates; this change has a `Changelog:` line).
- [ ] 6.3 `openspec validate nivistutor --strict`; archive; update beans-97jm
      (MVP done; note the expanding follow-ups: more tutorials, richer menu).
