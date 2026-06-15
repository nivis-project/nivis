# Overview

<!-- This is the canonical pitch + "how it works" prose. README links here; the
mdBook landing page (docs-site/src/index.md) {{#include}}s the anchored sections
below. Edit it here — nowhere else restates it. -->

<!-- ANCHOR: pitch -->
A Nix-native infrastructure tool where Terraform/OpenTofu **provider resources
are first-class Nix values**. A thin Go executor speaks the Terraform plugin
protocol directly to **unmodified provider binaries** — Nix is the configuration
frontend, Go is pure orchestration.

The headline capability — the reason this project exists — is the **round trip**:
a provider-created resource returns computed values (an IP, an ID, a generated
secret) back into Nix, which **re-evaluates** to produce dependent configuration,
repeating to a fixpoint. This is proven end to end across two providers with
unknown values originating on both sides.
<!-- ANCHOR_END: pitch -->

## How it works

<!-- ANCHOR: how-it-works -->
Nix evaluates your configuration to a JSON **IR** (`docs/IR-CONTRACT.md`). Values
that aren't known until apply-time are emitted as typed placeholders — a `__ref`
(a direct reference to another resource's output) or a `__derived` (a value Nix
*computed* from an output, e.g. a string built from an IP). The Go executor
ingests the IR, spawns the relevant provider binaries, drives
`GetProviderSchema`/`PlanResourceChange`/`ApplyResourceChange`, and collects the
real outputs into an **outputs ledger**. It then **re-evaluates Nix** with the
ledger injected, so placeholders resolve to concrete values; the new IR may
unlock more resources. This loop repeats to a **fixpoint** (no new value
resolves). Because each Nix-mediated (`__derived`) hop needs its own
re-evaluation, deep chains take more than two phases — the loop generalizes to
N phases. See `DESIGN.md` for why this (not an `Output<T>` promise model) is the
honest, Nix-shaped approach.
<!-- ANCHOR_END: how-it-works -->
