# Proposal: refresh-destroy-cli

## Why
The executor can resolve and apply a graph across phases (E3/E3.5), but it cannot
yet tear one down or reconcile drift, and it has no command-line surface. This
change adds the destroy and refresh engines and a `cobra` CLI, completing the two
headline-e2e assertions deferred from E4b (destroy in reverse order; refresh via
`ReadResource` without changing the plan) and turning the executor into a usable
`nixform` tool.

## What changes
- `internal/graph`: a `DestroyOrder()` helper returning resource ids in reverse
  dependency order (dependents before their dependencies).
- `internal/destroy`: for each resource in destroy order, call
  `ApplyResourceChange` with a null planned state (encoding the stored state as
  prior state), then remove it from the state store. Honor `lifecycle.preventDestroy`
  (refuse with an actionable error naming the resource).
- `internal/refresh`: for each resource in state, call `ReadResource` with its
  stored state and write back the reconciled state. Refresh does not plan or apply.
- `cmd/nixform`: a `cobra` CLI — `plan`, `apply`, `destroy`, `refresh`, and
  `state {list,show,rm}`, with a `--target <id>` filter and `--state <path>` /
  `--flake <ref>` options. `plan`/`apply` drive the phase loop (E3.5); `destroy`/
  `refresh` use the new engines.
- An e2e test extending the headline run: after apply, `refresh` leaves state
  unchanged (no plan delta) and `destroy` removes C, B, A in that order.

## Non-goals
- Remote state backends (the Store interface already admits them; not implemented).
- `--target` dependency-aware partial graphs beyond a simple id filter for the PoC.
- Real providers / registry (network-gated), schema codegen (E2).
- Rich plan rendering / colors — human-readable lines suffice for the PoC.

## Impact
- New: `internal/destroy`, `internal/refresh`, `cmd/nixform`, a `DestroyOrder`
  DAG helper, and an e2e for refresh+destroy. New dep: `spf13/cobra`.
- Completes E3b and the last two headline-e2e assertions, so the docs/TESTING.md
  headline test is fully satisfied. Gives the project a runnable entry point.
