# DESIGN.md — nixform architecture & decisions

This is the decision ledger. It exists so that a future session does not
re-derive (or undo) conclusions that were expensive to reach. Each decision
records the choice, the reasoning, and the alternative that was rejected.

## D1. Don't fork OpenTofu; drive the provider plugin protocol

**Decision.** Build a minimal Go engine that speaks the Terraform plugin
protocol (`tfprotov6`, over HashiCorp go-plugin/gRPC) to provider binaries. Use
`terraform-plugin-go` as the dependency and read OpenTofu's `internal/plugin`
as the reference for *how* to use it.

**Why.** Forking to "strip what we don't need" inverts the cost. The parser and
HCL loader are the small, easily-replaced parts (Nix replaces them). The
provider plugin client, state engine, dependency graph, and DAG scheduler are
the large parts — and we need those, so we'd inherit exactly the maintenance
burden we wanted to avoid. The protocol is stable; the config frontend is the
part we're actually changing.

**Rejected.** Forking OpenTofu and removing HCL. Higher ongoing cost, no upside.

## D2. Spawn unmodified providers; do not link (contrast with Pulumi)

**Decision.** Launch the upstream provider binary as a subprocess and talk
`tfprotov6` to it.

**Why / prior art.** The Pulumi Terraform Bridge is the closest prior art and is
worth mining — but it makes the **opposite** choice here, and the contrast is
instructive. Pulumi does *not* use provider binaries; it compiles the
provider's Go modules (against a forked plugin SDK) into its own provider
binary, per-provider, with a shim. That buys Pulumi tighter integration at the
cost of a per-provider build and a maintained SDK fork. Our headline goal is
**universal support for all existing providers with zero per-provider work**,
which spawn-not-link delivers directly. So we deliberately diverge from Pulumi
here. Do not refactor toward the link model.

**Mine from Pulumi instead:** its schema type-mapping (required/optional/
computed/sensitive, sets vs lists, nested blocks), its `ProviderInfo`/overlay
pattern (raw schema→code is usable but not idiomatic — plan an override seam),
and how it encodes *unknown* values to the provider during plan/diff (relevant
to D4 below).

## D3. Nix as a batch frontend; resolution by phased re-evaluation to a fixpoint

**Decision.** Nix evaluates configuration to a JSON IR. Cross-resource and
cross-domain references that aren't yet known are emitted as typed placeholders.
The Go executor applies what it can, collects real outputs, and the system
**re-evaluates Nix with those outputs injected**, repeating until a fixpoint
(no phase produces a new resolved value). Two phases is the shallow case; deeper
Nix-mediated dependency chains need more. We explicitly support **N phases**.

**Why.** This is the central constraint of the whole project. Nix evaluation is
a single forward batch pass that completes or errors — there is eval-time, then
build/apply-time, and they are separate. A value a provider computes at apply
(an IP, an ID, a generated secret) does not exist at eval time. Anything Nix
must *compute from* that value (a hostname string, a NixOS option, another
resource's input) therefore cannot be produced in the same evaluation. The only
faithful way to feed apply-time values back into Nix is to evaluate again with
them in scope.

**The two flavors of reference** (the executor must distinguish):
- **TF→TF:** resource A's output feeds resource B's input. Resolved *inside* the
  executor during apply; no re-eval needed.
- **\*→Nix:** a Nix expression computes something from an apply-time value (and
  that result may feed further resources). Requires re-eval with the value
  injected. This is what drives phase count.

**Why not Pulumi's elegant model.** Pulumi represents not-yet-known values as
`Output<T>` (a promise) and resolves them in-process as the program runs,
because a Pulumi program is a *live running process* the engine can feed values
back into. Nix has no promise, no suspend/resume, no live runtime to re-enter.
Pulumi's model is unavailable to us not because it's cleverer but because its
substrate is a different kind of thing. Our phased re-eval is the honest
Nix-shaped equivalent, not a workaround to feel bad about.

**Rejected (for now).** "Option B": a live evaluator the engine drives via
suspend/resume (libexpr internals). Elegant in theory, fragile and effectively
unsupported in practice. Our phased loop **converges toward** B's expressiveness
as iterations grow, without B's dependence on Nix internals. Revisit only if
re-eval cost becomes a *measured* problem.

## D4. The IR is the single frozen contract

**Decision.** `docs/IR-CONTRACT.md` defines the JSON IR. It is the API between
the Nix library (Epic 1), the codegen (Epic 2), and the executor (Epic 3/3.5).
Breaking changes require an OpenSpec change to the contract *first*.

**Why.** Three workstreams depend on it; once stable they can progress in
parallel. An underspecified linchpin is how this kind of project fragments.
The hard parts the contract must pin down — reference encoding (nested attrs,
list/set indices, refs inside `for_each`/`count`), `for_each`/`count` expansion
timing (Nix expands, executor receives concrete resources), unknown-value
representation toward the provider, and **how sensitive values cross the JSON
boundary without landing in world-readable `nix eval` output / the Nix store** —
are decided in the contract, not improvised per-epic.

## D5. Prove the round trip before building breadth

**Decision.** Critical path is: Nix lib core → IR contract → executor that
drives **one** (fake) provider through plan/apply → the phased-eval loop →
the two-provider e2e. General schema codegen for arbitrary providers and
registry integration come *after* the thesis is proven.

**Why.** The conceptual risk lives entirely in the round trip and the phased
loop. Codegen is breadth (how we reach "all providers"), not risk. Hand-written
constructors for the fake providers are enough to validate everything. Building
the generation machinery first means a lot of code before a single resource
round-trips.

## D6. Hermetic testing via in-repo fake providers

**Decision.** Write minimal Go binaries that *speak `tfprotov6`* and return
canned/computed values (no real APIs, no credentials, no network). The executor
drives them exactly as it would a real provider.

**Why.** Proves the protocol client and the whole pipeline deterministically and
offline — essential given the restricted network, and the right substrate for
the headline e2e. Real-provider runs are low conceptual risk and network-gated;
they are out of scope for the PoC and tracked as a separate bean.
