# ROADMAP.md: Nivis PoC

One **milestone** (the PoC / alpha base). Each epic below becomes a beans epic
under it (see `scripts/bootstrap-beans.sh`). Inside each epic, every task is an
OpenSpec change. The epic numbers are labels, **not** strict execution order;
follow the "Critical path" section.

## Milestone exit criterion (definition of done)

The headline e2e in `TESTING.md` passes: two providers, unknown values
originating on **both** sides, resolved across **≥3 phases**, with a Nix-side
consumer reading outputs from **both** providers. Everything else is in service
of making that test pass and trustworthy.

## Critical path (the order that actually matters)

```
E1 (Nix lib core: mkResource + refs + IR serializer)
        │
E1.5 ── IR CONTRACT  ← linchpin; write & freeze first   (IR-CONTRACT.md)
        │
E4a ── fake tfprotov6 providers (alpha, beta)  ← build early; test substrate
        │
E3a ── executor: ingest IR, spawn ONE fake provider, plan+apply, write state
        │
E3.5 ── PHASED EVALUATION TO FIXPOINT  ← the thesis
        │
E4b ── headline two-provider / unknowns-both-sides e2e  ← milestone exit
        │
(then breadth, off the critical path:)
E2 ── schema codegen   ·   E3b ── refresh/destroy/CLI polish   ·   E4c ── docs
```

Rationale in `DESIGN.md` D5. Do not build E2 (general codegen) before E3.5/E4b.

---

## Epic 1: Nix library core (`nivis-lib`)
Pure deterministic config layer. Emits the IR. No Go.

- **1.1 Resource constructor**: `mkResource { provider, type, name, config }`
  returning an attrset with stable identity, config, and a thunk exposing
  computed output attributes as referenceable Nix values (so dependency edges
  are implicit).
- **1.2 Reference system**: `resource.attr` access at eval time yields a typed
  placeholder the serializer recognizes as a cross-resource/cross-domain ref.
  Must survive a phase unresolved and be fillable on the next phase. See IR
  contract for encoding and the TF→TF vs \*→Nix distinction (DESIGN D3).
- **1.3 Meta-arguments**: `depends_on`, `lifecycle` (prevent_destroy,
  ignore_changes), `count`, `for_each`. `for_each` ↔ `builtins.mapAttrs`.
  **Expansion happens in Nix**: the IR carries concrete, already-expanded
  resources (decided in IR contract).
- **1.4 Module system**: NixOS-style `{ config, tf, pkgs, lib, ... }` via
  `lib.evalModules`, so user infra composes and resources across modules merge
  into one flat graph. This is where a NixOS/HM option can read a `tf.<res>.attr`.
- **1.5 IR serializer**: `toIR :: ResourceGraph -> JSON` emitting the canonical
  IR (types, provider, config with refs encoded, meta-args, edge list, the
  outputs-injection slot). Conforms to `IR-CONTRACT.md`.
- **1.6 Flake interface**: `nivis.plan`, `nivis.state`, `nivis.providers`
  outputs; `plan` accepts an injected-outputs argument (the phased-eval input).

## Epic 1.5: IR contract (linchpin) ⟵ write first
- **1.5a Author `IR-CONTRACT.md`**: the frozen JSON schema. Must pin:
  ref encoding (nested attrs, list/set indices, refs inside expansions);
  `for_each`/`count` expansion timing; unknown-value representation toward the
  provider; sensitive-value handling across the JSON/store boundary; the
  outputs-injection format consumed on re-eval. A fully-worked OpenSpec change
  for this already exists at `openspec/changes/define-ir-contract/`: implement
  it first; everything keys off it.

## Epic 2: Provider schema codegen (`nivis gen`)  ⟵ OFF critical path
Generates typed Nix constructors from live provider schemas. Build *after* E4b.
- **2.1** Go tool: spawn provider, handshake, `GetProviderSchema` → `schema.json`.
- **2.2** Schema → Nix type model (string/number/bool/list/set/map/object,
  computed/optional/required/sensitive). Mine Pulumi's type mapping (DESIGN D2).
- **2.3** Codegen: emit `<provider>/<type>.nix` constructors with required-field
  throws and optional passthrough. Plan an **override seam** (Pulumi overlay
  lesson): generated code is usable, not idiomatic.
- **2.4** Provider registry resolve/download/verify/cache. **Network-gated; not
  in PoC scope**, see CLAUDE.md §6. Track as its own bean; use fakes meanwhile.
- **2.5** Package codegen as a flake app (`nix run Nivis#gen -- --provider …`).

## Epic 3: Go executor (`Nivis`)
Pure orchestration. No HCL, no policy.
- **3a.1 IR ingestion**: read+validate IR → `ResourceNode`, `RefEdge`,
  `ProviderConfig`, `MetaArgs`.
- **3a.2 State backend**: trivial lockable local JSON state for the PoC. **No
  tfstate-format compatibility guarantee** (DESIGN: keep state trivial until the
  round trip works). Design the interface so remote backends can come later.
- **3a.3 Provider plugin manager**: spawn provider from a path, plain
  go-plugin/gRPC v6 handshake, pooled connections keyed by provider identity.
  (No muxer for the PoC.)
- **3a.4 DAG builder**: graph from `RefEdge`; parallel where possible; honor
  `depends_on`. Resolve **TF→TF** refs in-executor as outputs arrive.
- **3a.5 Plan engine**: per resource: `GetProviderSchema`, validate,
  `PlanResourceChange`, diff vs state, emit human-readable plan. No side effects.
  Unknown inputs presented to the provider per the IR contract's unknown repr.
- **3a.6 Apply engine**: `ApplyResourceChange`; write partial state after each
  success so a failed apply is recoverable.
- **3b.1 Refresh/destroy**: `ReadResource` reconcile; destroy ordering.
- **3b.2 CLI**: `plan/apply/destroy/refresh/state {list,show,rm}`, `--target`.

## Epic 3.5: Phased evaluation to fixpoint (the thesis)  ⟵ critical
Generalizes two-phase Option A to N phases. This is the epic the original
roadmap was missing.
- **3.5.1 Outputs ledger**: a JSON file accumulating resolved outputs across
  phases, in the injection format from the IR contract. World-readable-safe
  handling of sensitive outputs.
- **3.5.2 Phase driver**, loop:
  1. `nix eval .#nivis.plan` with the current outputs ledger injected
     (empty on phase 0).
  2. Ingest IR; via the DAG, find resources whose inputs are now fully known;
     plan+apply them; collect new computed outputs.
  3. Append new outputs to the ledger.
  4. If the phase resolved ≥1 new output **and** unresolved refs/resources
     remain → repeat. Else stop.
- **3.5.3 Fixpoint & error detection**: halt when a phase yields no new
  resolved value. If unresolved refs remain at halt → actionable error
  (unresolvable value or dependency cycle), naming the resources/attrs.
- **3.5.4 \*→Nix feedback**: verify a re-evaluated Nix expression (e.g. a NixOS
  option / a string built from a computed value) produces a concrete value in a
  later phase's IR and is consumable downstream. This is the round trip.

## Epic 4: Integration, e2e, DX
- **4a Fake providers** (build early): two in-repo `tfprotov6` providers,
  `provider-alpha` and `provider-beta`, with computed (unknown-at-plan) outputs.
  Spec in `TESTING.md`.
- **4b Headline e2e** (milestone exit): two providers, unknowns on both sides,
  ≥3 phases, Nix-side consumer reading from both. Full spec in `TESTING.md`.
- **4c Error UX**: Nix eval, schema validation, gRPC, and state-lock errors all
  produce actionable messages with resource identity, never raw stack traces.
- **4d Docs**: README, getting-started on the fake providers, IR contract and
  flake interface documented as stable contracts.
