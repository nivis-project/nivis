<div class="tn-hero">
  <img src="./banner.png" alt="Terrae Nivis — Infrastructure as Nix Code" />
</div>

# Terrae Nivis

**Infrastructure as Nix Code.** *(Terrae Nivis — Latin, "lands of snow";
formerly `nixform`.)*

A Nix-native infrastructure tool where Terraform/OpenTofu **provider resources
are first-class Nix values**. A thin Go executor speaks the Terraform plugin
protocol directly to **unmodified provider binaries** — Nix is the configuration
frontend, Go is pure orchestration.

The headline capability — the reason this project exists — is the **round trip**:
a provider-created resource returns computed values (an IP, an ID, a generated
secret) back into Nix, which **re-evaluates** to produce dependent configuration,
repeating to a fixpoint. This is proven end to end across two providers with
unknown values originating on both sides.

## How it works

Nix evaluates your configuration to a JSON **IR** ([the IR
contract](./ir-contract.md)). Values that aren't known until apply-time are
emitted as typed placeholders — a `__ref` (a direct reference to another
resource's output) or a `__derived` (a value Nix *computed* from an output). The
Go executor ingests the IR, spawns the relevant provider binaries, drives
`GetProviderSchema`/`PlanResourceChange`/`ApplyResourceChange`, and collects the
real outputs into an **outputs ledger**. It then **re-evaluates Nix** with the
ledger injected, so placeholders resolve to concrete values; the new IR may
unlock more resources. This loop repeats to a **fixpoint**. Because each
Nix-mediated (`__derived`) hop needs its own re-evaluation, deep chains take more
than two phases — the loop generalizes to N phases.

## Where to start

- **[Getting started](./getting-started.md)** — a hands-on walkthrough against
  the in-repo fake providers (offline, no credentials).
- **[Real providers (AWS)](./real-providers.md)** — drive a real provider end to
  end.
- **[Architecture & decisions](./design.md)** — why it is the way it is
  (spawn-not-link, batch-not-live, phased re-eval to a fixpoint).
- **[The IR contract](./ir-contract.md)** — the stable interface between the Nix
  frontend and the Go executor.
