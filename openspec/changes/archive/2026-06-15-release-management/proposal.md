# Proposal: release-management

## Why
The project has no release process and the version is **inconsistent and wrong**:
`cmd/tn/splash.go` hardcodes `v1.0` while `flake.nix` hardcodes `0.1.0` — they
disagree, and `v1.0` is nonsense for a PoC. There is no changelog, no way to cut a
release, and no single source of truth for the version (beans-ohkv).

We want: one canonical version, a changelog, a simple `patch/minor/major` bump,
and an automated GitHub release — jj/git compatible.

## What changes
- **A `VERSION` file is the single source of truth.** Top-level `VERSION`
  containing a bare semver. Initial value **`0.2.0`** — `0.1.x` was the PoC
  (complete); `0.2.x` begins "for real."
  - `flake.nix` reads it (`lib.fileContents ./VERSION` / `builtins.readFile`) and
    injects it into the binaries via `-ldflags -X main.version=<v>`, replacing the
    hardcoded `version = "0.1.0"`.
  - The Go side keeps a `version` variable whose default is read from an embedded
    `VERSION` (`//go:embed`), so a plain `go build` (no ldflags) also reports the
    canonical version. `tn --version` and the splash show it everywhere.
- **`CHANGELOG.md`** in Keep-a-Changelog format: an `Unreleased` section plus a
  `0.1.0` entry summarizing the PoC (round trip, real providers, update/replace,
  docs site).
- **`scripts/release.sh patch|minor|major`** — the "cut a release" UX, offline and
  jj/git compatible:
  - reads `VERSION`, computes the next semver,
  - writes `VERSION`, rolls the `CHANGELOG.md` `Unreleased` section into a dated
    `[<new>]` section,
  - commits (`jj describe`/`jj new`, falling back to `git commit`) and creates a
    `v<new>` **git tag**, then pushes the tag.
  - `--dry-run` prints the plan without writing.
- **`.goreleaser.yaml` + `.github/workflows/release.yml`** — the publish step:
  on a pushed tag `v*`, goreleaser cross-builds `tn` and `tn-gen`
  (linux/darwin × amd64/arm64) with the version from the tag, produces checksums
  and changelog-derived notes, and creates the **GitHub release** with the
  binaries attached.
- **`docs/RELEASING.md`** documenting the flow (`release.sh minor` → tag → CI
  publishes), referenced from the docs.

## Non-goals
- Publishing to other channels (Homebrew, nixpkgs, container registries) —
  goreleaser can do these later; out of scope now.
- Cutting an actual release in this change — we add the machinery and verify it
  (goreleaser `check` + a local snapshot build, `release.sh --dry-run`); a real
  `v0.2.0` tag is a deliberate follow-up the user triggers.
- Signing/SBOM — future hardening.

## Impact
- New: `VERSION`, `CHANGELOG.md`, `scripts/release.sh`, `.goreleaser.yaml`,
  `.github/workflows/release.yml`, `docs/RELEASING.md`. Changed: `flake.nix`
  (version from VERSION + ldflags), `cmd/tn/splash.go` (version default from
  embedded VERSION).
- Verification: `nix run .#tn -- --version` and `go build` both report `0.2.0`;
  `goreleaser check` passes and a `--snapshot` build produces the binaries;
  `release.sh --dry-run` shows a correct bump. No tag is pushed.
- Closes beans-ohkv.
