# CLAUDE.md — Nivis

> You (Claude Code, "cc") are the autonomous builder **and** the project manager
> for this repository. You administer **beans** (the issue tracker) at the
> milestone/epic level and drive **OpenSpec** for every unit of work inside an
> epic. Read this whole file before doing anything else.

## 0. First actions, every session (in order)

1. Run `beans prime` and heed its output. (Beans is the source of truth for what
   to work on next.)
2. Run `openspec list` to see active/proposed/archived changes.
3. Read `docs/DESIGN.md` (the *why* — architecture decisions you must not regress)
   and `docs/ROADMAP.md` (the *what* — epics mapped to beans).
4. Read `docs/IR-CONTRACT.md`. The IR is the frozen contract between all epics.
   Treat it as an API: a breaking change requires an OpenSpec change first.

## 1. What we are building

`Nivis`: a Nix-native infrastructure tool where Terraform/OpenTofu **provider
resources are first-class Nix values**, driven by a thin Go executor that speaks
the Terraform plugin protocol (`tfprotov6`) directly to **unmodified provider
binaries**. Nix is the configuration frontend; Go is pure orchestration.

The headline capability — the reason this project exists — is the **round trip**:
provider-created resources return computed values back into Nix, which
re-evaluates to produce dependent configuration. Proving that round trip across
two providers, with unknown values originating on both sides, is the PoC's
definition of done.

## 2. Non-negotiable architecture invariants

These were decided deliberately (see `docs/DESIGN.md` for the reasoning). Do **not**
"helpfully" refactor away from them:

- **Do not fork OpenTofu.** Drive providers via `tfprotov6` over go-plugin/gRPC.
  Read OpenTofu/`terraform-plugin-go` as the protocol reference; do not vendor a
  stripped core.
- **Spawn, do not link.** We launch *unmodified* upstream provider binaries and
  talk the protocol to them. We do **not** statically link patched provider Go
  source the way the Pulumi bridge does. Universal, zero-per-provider support is
  the goal; that is what spawn-not-link buys us.
- **Nix is a batch evaluator, not a live runtime.** We resolve values by
  **phased re-evaluation to a fixpoint** (generalized "Option A"), not by an
  in-process promise/`Output<T>` model. Pulumi gets the live model for free
  because its programs are real running processes; Nix cannot, and we do not
  pretend otherwise.
- **The IR is frozen.** `docs/IR-CONTRACT.md` is the contract. Change the spec
  before changing the shape.

## 3. The working loop (beans + OpenSpec)

You operate at two levels. Keep them distinct.

**Beans = milestones & epics (project memory / audit trail).**
- The PoC is one **milestone**. Each roadmap epic is a beans **epic** that is a
  child of the milestone. Larger sub-pieces may be beans **features** under an
  epic.
- You own beans administration: create/update epics, set status, pick the next
  unblocked epic, and record discovered work.
- **Discovered work goes to beans immediately** (`beans create ... --tag
  discovered`), naming the bean you were working on. Never drop it under context
  pressure.
- When you finish work tied to a bean, reference it in the commit:
  `... Closes beans-XXXX.`

**OpenSpec = the tasks inside an epic (the contract before the code).**
- For each task in an epic, create an OpenSpec change:
  `openspec/changes/<change-id>/` with `proposal.md`, `tasks.md`, and spec
  deltas under `specs/` (ADDED/MODIFIED/REMOVED requirements in
  GIVEN/WHEN/THEN form).
- `openspec validate <change-id>` must pass before you write implementation.
- **Do not write implementation code until that change's spec is written and
  self-consistent.** This is a hard gate.
- Implement one change at a time. When done and tests pass, archive it
  (`openspec archive <change-id>`) and update the linked bean.

The mapping: **one beans epic → one or more OpenSpec changes.** Record the
OpenSpec change-ids in the body of the beans epic so the two stay linked.
See `openspec/changes/define-ir-contract/` for a fully worked example to mirror.

## 4. Order of work

Follow `docs/ROADMAP.md`. The critical-path ordering is **not** strict 1→2→3→4.
Specifically: prove a single-provider round trip and the phased-eval loop
**before** building general schema codegen. Codegen is how we scale to "all
providers" later; it is not on the path to validating the thesis. The roadmap
encodes the correct order — trust it over epic numbering.

## 5. Testing is part of "done"

No change is complete without tests. See `docs/TESTING.md`.
- Pure Nix functions: property tests.
- Go: table-driven unit tests.
- Integration & e2e: against **in-repo fake `tfprotov6` providers** (hermetic,
  no network, no credentials). Build these fakes early — they are how every
  later epic is tested.
- The milestone exit criterion is the headline e2e in `docs/TESTING.md`: **two
  providers, unknown values originating on both sides, resolved across ≥3
  phases, with a Nix-side consumer reading outputs from both providers.**

## 5.5 Documentation is part of "done" (the docs-coverage gate)

Just as no change is complete without tests, no change is complete without
asking whether the **docs kept up**. Before you `openspec archive` a change, run
the rubric in `docs/DOCS-GATE.md` and decide one of: **new document**, **new
paragraph/section**, **modifications only**, or **none**. A new user-facing
*concept or capability a user searches for by name* (variables, datasources,
remote state) usually wants its **own document** in `docs/`, surfaced on the site
(`docs-site/src/<topic>.md` `{{#include}}`-ing it + a `SUMMARY.md` entry).

Record the decision as a `Docs impact:` line in the change's `proposal.md`.
`tests/check-docs-gate.sh` (run inside `tests/check-docs-ssot.sh`) fails if any
in-scope change lacks that line. The script does not judge the content; it
forces the call to be made and written down. This is the docs analogue of §5.

## 6. Environment constraints (read before you hit a wall)

- Network egress is **restricted to an allowlist** (GitHub, the Go module proxy
  via GitHub, PyPI, npm, crates). The **OpenTofu provider registry and the Nix
  binary cache are NOT reachable.** Consequences:
  - Do not implement provider download from the registry as a blocking step in
    the PoC. Use the in-repo fake providers instead. If real-provider download
    is needed later, it is a separate, network-gated task — create a bean and
    flag it; do not silently fail.
  - `terraform-plugin-go` is fetchable via the Go proxy/GitHub — fine to depend
    on.
  - If a Nix evaluator is not present, that is a known setup gap: record a bean
    and prefer a vendored/static evaluator or a stub IR emitter for early
    phases rather than blocking.
- If a network call fails with an `x-deny-reason`, the domain is not allowlisted;
  surface that clearly rather than retrying blindly.

## 7. Stack

Go 1.22+ (executor, codegen tool, fake providers), Nix (library + codegen
templates), gRPC via `terraform-plugin-go`/`buf`, `cobra` for the CLI.
