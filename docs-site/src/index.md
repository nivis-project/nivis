<div class="tn-hero">
  <img src="./banner.png" alt="Terrae Nivis — Infrastructure as Nix Code" />
</div>

# Terrae Nivis

**Infrastructure as Nix Code.** *(Terrae Nivis — Latin, "lands of snow";
formerly `nixform`.)*

{{#include ../../docs/OVERVIEW.md:pitch}}

## How it works

{{#include ../../docs/OVERVIEW.md:how-it-works}}

## Where to start

- **[Getting started](./getting-started.md)** — a hands-on walkthrough against
  the in-repo fake providers (offline, no credentials).
- **[Real providers (AWS)](./real-providers.md)** — drive a real provider end to
  end.
- **[Architecture & decisions](./design.md)** — why it is the way it is
  (spawn-not-link, batch-not-live, phased re-eval to a fixpoint).
- **[The IR contract](./ir-contract.md)** — the stable interface between the Nix
  frontend and the Go executor.
