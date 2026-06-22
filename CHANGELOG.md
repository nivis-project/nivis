# Changelog

All notable changes to Nivis are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims to
follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

`0.1.x` was the proof-of-concept milestone; `0.2.x` begins development "for real."

## [Unreleased]

### Added
- Added an optional `backend` field to the IR (and `toIR`/`toModuleIR`) declaring
  where state is stored (e.g. an s3 `bucket`/`key`/`region`), so a remote state
  backend is configured in the Nix flake rather than via flags or env vars. It is
  static config (no refs, no credentials); absent means the local file store. This
  is the contract groundwork for the S3 backend (M2/B1); no backend is implemented
  yet.

## [0.4.4] - 2026-06-19

### Changed
- Changed the `nivistutor` tutorial menu order so getting-started is always listed
  first, followed by the per-release feature tutorials newest-first (a numeric
  version compare), instead of a plain alphabetical sort.

### Fixed
- Fixed `nivis apply` always printing `+ create` for every applied resource: it
  now shows the real op (`+` create, `~` update, `-/+` replace, `=` no-op), with a
  datasource still shown as `r` read. The state machine was already correct (an
  in-place update or no-op was never a re-create); only the report was wrong.
- Fixed `nivis plan` reporting a spurious `~ update` for a resource whose config
  reads a datasource: plan now reads the (side-effect-free) datasources first, so
  an unchanged datasource-dependent resource reports no-op (`=`) instead of an
  update. (Surfaced by the features-0.4 tutorial.)

## [0.4.3] - 2026-06-19

### Fixed
- Fixed `nivis gen` emitting a duplicate `name` formal when a provider attribute
  is named `name` (also `overrides`/`nivis`), which made the generated `.nix`
  invalid (`error: duplicate formal function argument 'name'`). The colliding
  attribute is now accepted under a `cfg_<name>` alias and emitted into `config`
  under its real key, while the Nivis instance name still threads to mkResource.
  This unblocked a large class of real-provider constructors (the nivis-registry
  PoC saw 1379 of 2414 fail on this, mostly azurerm and google `*_iam_policy`).

### Added
- Added `nivistutor`, a companion CLI (flake app `#tutor`) that scaffolds a chosen
  tutorial's starter files (a ready `flake.nix`, the config, and a README) into
  your own directory so you read, edit, and run them with plain `nivis` (no
  `--flake`/`--attr`); it does not run nivis for you. Ships a **getting-started**
  tutorial and a **features-0.4** tutorial, embedded in the binary (offline,
  version-locked). `nix shell …#nivis …#tutor` carries nivis, nivistutor, and the
  fake providers a scaffolded tutorial needs. Tutorials are now self-contained
  starter directories under `nix/example/tutorial-<name>/`.

## [0.4.2] - 2026-06-19

### Fixed
- Fixed `nivis gen` configuring the provider before fetching its schema, which
  broke codegen for credential-requiring providers (proxmox, azurerm, google):
  schema extraction now uses a configure-free client path, so `GetProviderSchema`
  is fetched without calling `ConfigureProvider`. `plan`/`apply`/`refresh`/
  `destroy` still configure as before. Reported by the nivis-registry companion
  project.

## [0.4.1] - 2026-06-19

### Added
- A `fake-providers` flake package, so the offline tutorials run with a single
  `nix shell .#nivis .#fake-providers` (no `go build`): both in-repo fake
  providers land on `PATH`, and the example configs reference them by bare name.

### Changed
- The offline docs (getting-started, the feature tutorial) are Nix-first: enter
  `nix shell .#nivis .#fake-providers` and run `nivis …`, rather than building
  binaries with the Go toolchain (kept as a contributor fallback).

## [0.4.0] - 2026-06-18

The **Road to v1** milestone (M1): the daily-driver features, so a Nix developer
can manage a real, multi-resource project end to end without dropping back to
Terraform. See `docs/TUTORIAL-FEATURES.md` for a hands-on, no-cloud tour and
`docs/releases/release-0.4/release-notes-0.4.md` for the milestone notes.

### Added
- **Variables** (`nivis.mkVars`): declare typed config variables (`str`/`int`/
  `bool`/`any`) with defaults; required when no default. Set them with `--var
  name=value`, `--var-file <json>`, or `NIVIS_VAR_<name>`, with Terraform
  precedence (an explicit `--var` wins). String values are coerced to the declared
  scalar type, so `--var replicas=5` satisfies an `int` var.
- **Datasources** (`nivis.mkData`): read existing infrastructure (an AMI, a VPC, a
  lookup) and feed it into resources. Read per phase, so a datasource may depend on
  a resource's apply-time output (it rides the round trip). Never planned, applied,
  or written to state.
- **Stack outputs**: declare named values with the `outputs` argument to `toIR`
  and read them with **`nivis output [name]`** (human-readable, a single value, or
  `--json` for a CI step / another stack).
- **Shell completion**: `nivis completion <bash|zsh|fish|powershell>`, with dynamic
  completion of resource ids (from state) for `state show`, `state rm`, and
  `--target`.
- **State pull/push**: `nivis state pull` / `nivis state push` move the whole state
  document (the seam a remote backend will reuse); `push` confirms before
  overwriting and requires `--force` when non-interactive.
- **Codegen now emits nested blocks**: `nivis gen` constructors include a
  resource's nested blocks with the correct list-vs-single shape, so the generated
  constructor doubles as the per-provider argument reference.
- **Docs**: a hands-on feature tutorial (against the in-repo fakes, no cloud),
  Variables and Datasources reference pages, a comparison page vs other IaC tools,
  and a forward-looking roadmap.

### Changed
- **Plan/apply/destroy output is colorised by change type and grouped by phase**
  (`+` create, `~` update, `-/+` replace, `-` destroy, `=` no-op, `r` datasource
  read), so the phased fixpoint is visible. Respects `NO_COLOR` and non-TTY output.
- State commands report clearly: `state list` notes when empty, `state rm` of a
  missing id says so, and a held state lock now times out with an actionable
  message instead of hanging.

### Fixed
- Nested-block shape mistakes (a list-nested block written as a bare attrset) now
  produce an actionable error naming the attribute and the fix, instead of a
  cryptic codec error.

## [0.3.1] - 2026-06-15

## [0.3.0] - 2026-06-15

### Added
- Resource lifecycle: a second apply now updates in place or replaces
  (destroy + create for force-new attributes) instead of always creating; honors
  `prevent_destroy` on replace.
- AWS S3 tutorial: an `aws_s3_object` whose content is generated by Nix from the
  bucket's apply-time output, the round trip across the Nix/Terraform domains.
- Release management: a `VERSION` single source of truth, this changelog,
  `scripts/release.sh`, and a goreleaser GitHub release workflow.

## [0.1.0] - 2026-06-15

The proof-of-concept. Proves the thesis end to end.

### Added
- **The round trip**: provider-created values feed back into Nix, which
  re-evaluates to a fixpoint across N phases (proven across two providers with
  unknown values originating on both sides).
- **Nix library**: `mkResource`, `mkProvider`, references (`__ref`/`__derived`),
  `toIR`, `count`/`for_each` expansion, and a module system.
- **Go executor**: IR ingest/validate, DAG + TF→TF resolution, the phased-eval
  loop, plan/apply/destroy/refresh, and schema codegen (`tn-gen`).
- **Provider protocol**: drives unmodified provider binaries over `tfprotov5`
  and `tfprotov6` via go-plugin; provider config (incl. nested blocks) expressed
  in Nix.
- **Real providers**: registry fetch + checksum verification; proven against AWS.
- **CLI** `tn` with a branded splash; **flake apps** (`nix run .#tn`).
- **Docs**: a branded mdBook site deployed to GitHub Pages, a from-scratch AWS
  S3 tutorial, the IR contract, and the design/testing docs.

[Unreleased]: https://github.com/wearetechnative/nivis/compare/v0.4.4...HEAD
[0.4.4]: https://github.com/wearetechnative/nivis/releases/tag/v0.4.4
[0.4.3]: https://github.com/wearetechnative/nivis/releases/tag/v0.4.3
[0.4.2]: https://github.com/wearetechnative/nivis/releases/tag/v0.4.2
[0.4.1]: https://github.com/wearetechnative/nivis/releases/tag/v0.4.1
[0.4.0]: https://github.com/wearetechnative/nivis/releases/tag/v0.4.0
[0.3.1]: https://github.com/wearetechnative/nivis/releases/tag/v0.3.1
[0.3.0]: https://github.com/wearetechnative/nivis/releases/tag/v0.3.0
[0.1.0]: https://github.com/wearetechnative/nivis/releases/tag/v0.1.0
