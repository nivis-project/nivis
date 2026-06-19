# ROADMAP.md: Nivis

Nivis cleared its **proof-of-concept milestone**. The thesis is proven: real
Terraform/OpenTofu provider resources are first-class Nix values, driven by a
thin Go executor that spawns unmodified provider binaries, and provider outputs
round-trip back into Nix across phases to a fixpoint. On top of that we have real
AWS apply/update/replace/destroy, schema codegen, and an end-to-end "build a
NixOS AMI and launch it" example.

That makes Nivis **experimental / alpha** (`0.3.x`): real, but small. This
roadmap is about the next thing, taking Nivis from "the demo works" to "I can run
my real infrastructure on this," and eventually to something an enterprise can
adopt. The PoC roadmap that got us here is preserved at the bottom as history.

> **How this maps to beans.** Each phase below is a beans **milestone**; each
> theme under it is a beans **epic**; each task inside an epic is an OpenSpec
> change (spec before code). See `CLAUDE.md` §3. The doc is the *why and what*;
> beans is the audit trail.

## Where we are honestly weak

`docs/COMPARISON.md` states this plainly. Versus Terraform/OpenTofu, Pulumi and
CDK, the gaps that actually block adoption today are:

- **State is local-only.** There is a `Store` interface seam, but no shared
  backend and no locking. Two people (or CI) cannot safely touch the same infra.
- **No variables / overrides.** Config is whatever the flake hard-codes plus an
  ad-hoc `ledger.vars`. There is no first-class way to parameterise per
  environment or pass values at the CLI.
- **No datasources.** The provider protocol's `ReadDataSource` is unused; you
  cannot look up an existing AMI, VPC, or zone the way every other tool can.
- **Thin DX.** Plan/apply/destroy output is not colorised by change type, there
  is no shell completion, and there is no per-provider reference documentation.
- **No enterprise controls.** No policy-as-code, no RBAC/audit, no hosted control
  plane, and provider download from the registry is network-gated and not the
  default path.

The phases close these in the order that unlocks the widest audience soonest.

## Architecture invariants (do not regress)

Every phase below is bound by `docs/DESIGN.md`. In particular: **spawn unmodified
providers, do not link** them; Nix is a **batch evaluator** resolved by phased
re-evaluation to a fixpoint, not a live `Output<T>` runtime; **the IR is the
frozen contract** (`docs/IR-CONTRACT.md`), so any feature that changes the IR
shape needs an OpenSpec change to the contract first; and tests run against
**in-repo fake providers** (hermetic, no network, no credentials).

---

## Phase A: a daily-driver for Nix developers  ⟵ the next milestone

> Beans milestone: `nixform2-zdj0` ("Road to v1"). Epics: A1 `nixform2-kym5`,
> A2 `nixform2-6e6i`, A3 `nixform2-yqd3`, A4 `nixform2-oycy`, A5 `nixform2-n2rg`,
> A6 `nixform2-z8e1`.

**Definition of done:** a Nix developer can manage a real, multi-resource project
end to end, day to day, without dropping back to Terraform, with shared state,
parameterised config, datasource lookups, and a plan they can actually read. This
is the headline goal for the next milestone, the same role the round-trip e2e
played for the PoC.

- **A1. Variables and overrides.** First-class inputs to a plan: typed variables
  with defaults, a CLI way to set them (`--var`, `--var-file`), and a clear
  precedence (defaults < file < flag < environment). Must thread cleanly through
  the phased-eval loop (the ledger already carries `vars`; formalise it) and stay
  pure: no impurity sneaks into the Nix evaluation. Probably an IR-contract touch
  for how vars enter the `plan` function.
- **A2. Datasources.** Drive the provider protocol's `ReadDataSource` so a config
  can read existing infrastructure (an AMI by filter, a VPC, an availability
  zone) and feed it into resources. Needs a Nix-lib constructor (`mkData` or
  similar), executor support, and an IR-contract addition for the datasource node
  and its outputs. Datasource reads happen per phase like any other node.
- **A3. Legible plan/apply/destroy output.** Colorise by change type
  (`+ create`, `~ update`, `-/+ replace`, `- destroy`, `= no-op`), summarise
  counts, and make the phased nature visible (which resources resolved in which
  phase). Respect `NO_COLOR` and non-TTY output. No behaviour change, pure DX.
- **A4. Shell completion.** Cobra can generate bash/zsh/fish completion; wire it
  up (`nivis completion <shell>`) and complete resource ids for
  `state show` / `--target` from the state file.
- **A5. Per-provider reference docs.** Today a user reads the provider's Terraform
  registry docs and mentally translates HCL to Nivis. Generate or curate a
  "Terraform docs to Nivis" mapping so `aws_instance`'s arguments are discoverable
  in Nivis terms. Couples naturally to schema codegen (Epic 2, already built).
- **A6. State ergonomics.** Configurable state path is done; add the small
  things a real project needs: `state list/show/rm` polish, a `state pull/push`
  shape that the remote backend (Phase B) will reuse, and clear errors on a
  stale or locked state file.

## Phase B: team-ready  ⟵ after Phase A

> Beans milestone: `nixform2-kovh`. Epics: B1 `nixform2-izhk`,
> B2 `nixform2-0oqk`, B3 `nixform2-tyzs`, B4 `nixform2-cdfj`.

**Definition of done:** multiple people and CI can safely operate the same
infrastructure concurrently.

- **B1. Remote state backend (S3 first).** Implement the `Store` seam against S3
  (object per state, server-side encryption, the credential chain Nivis already
  uses). Keep the format Nivis's own; **no tfstate compatibility guarantee**
  (DESIGN). Configured in the flake, not via env soup.
- **B2. State locking.** A lock so two concurrent applies cannot corrupt state
  (DynamoDB-style advisory lock for the S3 backend, with a `force-unlock` escape
  hatch and clear "who holds the lock" errors).
- **B3. Drift detection.** `refresh` exists; build a real "plan shows drift"
  experience that reconciles remote reality against stored state and surfaces
  out-of-band changes.
- **B4. Multiple environments.** A clean pattern for dev/staging/prod from one
  config: workspaces or per-environment var-files + state keys, decided in a
  spec, not improvised.

## Phase C: enterprise-credible  ⟵ the longer horizon

> Beans milestone: `nixform2-1okn`. Epics: C1 `nixform2-alr9`,
> C2 `nixform2-84fs`, C3 `nixform2-m83a`, C4 `nixform2-q7fx`, C5 `nixform2-7evo`.

NixOS is gaining enterprise traction; this is where Nivis earns a seat there.
These are deliberately later, after the basics are solid, and several are large
enough to be their own milestones.

- **C1. Policy as code / guardrails.** A pre-apply policy hook (deny by rule,
  required tags, allowed regions). Evaluate doing this *in Nix* (assertions in
  the module system) versus an external engine; Nix-native is the differentiator.
- **C2. RBAC, teams, audit.** The story for who can apply what, and an audit
  trail. Likely pairs with a remote backend and possibly a hosted control plane;
  scope carefully, this is where tools grow a SaaS.
- **C3. Provider registry integration.** Real provider download/verify/cache from
  the OpenTofu registry. **Network-gated** (CLAUDE.md §6); today providers are
  fetched on first use but this needs hardening, offline/air-gapped mirrors, and
  supply-chain verification for enterprise.
  > **Companion project:** a separate **`nivis-registry`** project has been
  > started to own (some or all of) this. The exact split between `nivis-registry`
  > and the in-repo C3 epic (`nixform2-m83a`) is not decided yet; once it is, this
  > entry and the bean will be reconciled (C3 may move wholesale to the companion
  > project or keep only the client-side integration here).
- **C4. Secrets at scale.** The IR already keeps sensitive values out of the
  world-readable store; extend to integration with real secret stores (Vault,
  SSM, sops-nix) so secrets never transit the Nix store at all.
- **C5. Scale and performance.** Phased re-eval cost on large graphs is currently
  unmeasured. Measure it; optimise only if it is a *measured* problem (DESIGN
  rejects premature cleverness like a live evaluator).

## Cross-cutting, every phase

- **Stay hermetic.** Every feature lands with tests against the in-repo fakes; the
  fakes grow new capabilities (a datasource-serving fake, a drift-injecting fake)
  as the features that need them arrive.
- **Keep the lib pure.** `nivis.lib` stays builtins-only (no nixpkgs); only
  packages/apps may force nixpkgs.
- **Spec before code.** IR-affecting work (vars entry, datasource node) updates
  `IR-CONTRACT.md` via an OpenSpec change first.

---

## History: the PoC milestone (delivered)

Kept for the record. This is the roadmap that proved the thesis; every epic below
is complete (see the beans milestone `nixform PoC / alpha base` and its epics).

**Milestone exit criterion (met):** the headline e2e in `TESTING.md`: two
providers, unknown values originating on **both** sides, resolved across **≥3
phases**, with a Nix-side consumer reading outputs from **both** providers.

**Critical path that was followed:**

```
E1 (Nix lib core: mkResource + refs + IR serializer)
        │
E1.5 ── IR CONTRACT  (linchpin; written & frozen first)
        │
E4a ── fake tfprotov6 providers (alpha, beta)  (test substrate)
        │
E3a ── executor: ingest IR, spawn ONE fake provider, plan+apply, write state
        │
E3.5 ── PHASED EVALUATION TO FIXPOINT  (the thesis)
        │
E4b ── headline two-provider / unknowns-both-sides e2e  (milestone exit)
        │
(then breadth:) E2 schema codegen · E3b refresh/destroy/CLI · E4c/4d error UX & docs
```

**Epics delivered (PoC and the alpha follow-ons):**

- **E1** Nix library core: `mkResource`, `mkProvider`, the reference system,
  meta-arguments (`depends_on`, `lifecycle`, `count`/`for_each` expanded in Nix),
  the module system, `toIR`, and the flake interface (`nivis.plan`).
- **E1.5** The IR contract: `IR-CONTRACT.md` + `ir-schema.json`, the frozen JSON
  contract pinning ref encoding, expansion timing, unknown representation, and
  sensitive-value handling.
- **E2** Provider schema codegen (`nivis gen`): typed Nix constructors from a
  provider's `GetProviderSchema`.
- **E3** Go executor: IR ingestion, the lockable local `Store`, the plugin
  manager (spawn + gRPC handshake, v5 and v6), the DAG, plan and apply engines.
- **E3.5** Phased evaluation to fixpoint: the outputs ledger, the phase driver,
  fixpoint and cycle detection, and verified `*→Nix` feedback (the round trip).
- **E3b** Refresh and destroy engines and CLI (`plan/apply/destroy/refresh/state`,
  `--target`, `--refresh`, `--build`).
- **E4a** Fake `tfprotov6`/`tfprotov5` providers (the hermetic test substrate).
- **E4b** The headline two-provider, unknowns-both-sides, ≥3-phase e2e.
- **E4c/4d** Error UX and docs (actionable errors; README, getting-started, the
  stable-contract docs).
- **Real-provider support** (M2): real `tfprotov5` + on-first-use registry fetch,
  proven against AWS.
- **Resource lifecycle:** update and replace beyond create-only, with
  `prevent_destroy`.
- **EC2 + NixOS:** build a NixOS AMI in Nix and launch it through Nivis, with
  `nivis apply` realising the image itself (`__build` / `nivis.drv`).
- **Branding, rename to Nivis, release management** (versioning, changelog,
  releases, the docs site).
