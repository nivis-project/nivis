# Proposal: auto-build-derivations

## Why
When a resource's config value is a Nix build output — e.g. the EC2 tutorial's
`aws_s3_object.source = the built NixOS .vhd` — `nivis` fails at apply with
`opening S3 object source (...): no such file or directory`. The cause: `nivis`
resolves config via `nix eval`, which **evaluates but does not realise
(build)** derivations. The store path exists in the IR but the file was never
built. Today the user must `nix build .#ec2-image` *before* `nivis apply`
(beans-qcwb) — an anti-pattern: the tool already drives Nix, so it should build
what it needs itself.

Crucially, "what needs building" is **not knowable up front**. A Nix build is just
another node in the phased fixpoint: Nix builds A → Nivis applies it, producing
output `a` → `a` feeds a Nix expression that builds B → Nivis applies B,
producing `b` → `b` feeds a build of C → … Each phase re-evaluates with the
accumulated ledger, so a later build only becomes evaluable once an earlier
resource's output exists. The executor must therefore build **only what the
resources ready *this* phase reference**, then apply, then re-eval to discover the
next phase's build needs.

## What changes
- **A `__build` typed leaf, authored via `nivis.drv`.** Instead of a bare,
  fragile interpolation (`"${image}/${image.passthru.filePath}"`), the author
  writes `source = nivis.drv image;` (optionally `nivis.drv image { file =
  "..."; }`). The Nix lib emits a `{ "__build": { "path": "<absolute store
  path>" } }` leaf into the IR. This is **explicit** — it marks "this value is a
  build output the executor must realise" — consistent with the existing typed
  leaves (`__ref`/`__derived`), not string-guessing.
- **The executor realises `__build` leaves per resource, before apply.** In
  `applyOne`, after a resource's config is resolved (its `__ref`/`__derived`
  inputs known) and before plan/apply, the executor walks the config for
  `__build` leaves, realises each store path that is not already valid
  (`nix-store --realise <store-root>`), and substitutes the concrete path string.
  Because `applyOne` runs per ready resource each phase, this builds **only
  what's needed now** and supports the A→a→B→b→C cross-boundary chain — the
  build becomes another apply-time dependency the fixpoint resolves.
- **A `__build` leaf is a *known* value, not an unknown placeholder.** Unlike
  `__ref`/`__derived` it does not participate in ledger resolution, edges, or
  unknown-after-apply: the path is known at eval; only the *file* needs realising.
  It passes through `resolve`/`toIR` unchanged and is acted on by the executor.
- **Opt-out + clear errors.** A `--no-build` flag (default off) skips realising
  (for CI that pre-builds or offline runs); a build failure surfaces a clear
  error naming the store path/derivation.
- **Migrate the EC2 example + tutorial** to `nivis.drv` and drop the
  `nix build .#ec2-image` step (one `nivis apply` builds and ships).

## Non-goals
- Building things no resource references (only what's reachable from a ready
  resource's config is built — never "everything").
- A general Nix-build scheduler — the phased loop already orders this; `__build`
  rides it.
- Remote builders / substituter config — `nix-store --realise` uses the ambient
  Nix configuration.

## Impact
- New: `nivis.drv` helper + `__build` leaf (`nix/lib/ref.nix`, exported from
  `default.nix`); ingest classifies `__build`; the executor realises + substitutes
  in `applyOne`; `--no-build` on plan/apply; IR-CONTRACT documents the leaf.
- Changed: `nix/example/ec2.nix` + `docs/TUTORIAL-EC2-NIXOS.md` use `nivis.drv`
  (drop the manual `passthru.filePath` and the build-first step).
- Tests: Nix property (the `__build` leaf shape + passes through resolve); Go
  (executor realises + substitutes via a stub realiser; `--no-build` skips);
  re-verify the EC2 tutorial live — a single `nivis apply` builds the image and
  serves HTTP 200.
- Closes beans-qcwb; completes epic beans-uqn6.
