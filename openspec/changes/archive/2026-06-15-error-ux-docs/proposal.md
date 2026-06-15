# Proposal: error-ux-docs

## Why
The PoC works end to end, but its rough edges are error presentation and docs.
This change closes E4c (actionable errors with resource identity, never raw
stack traces) and E4d (README, getting-started, documented contracts), making the
project trustworthy to operate and approachable to a newcomer. It is the final
milestone epic — breadth/polish, no new core behavior.

## What changes
- Error UX at the CLI boundary:
  - Silence cobra's usage-dump on runtime errors (usage is for flag misuse only);
    print `error: <message>` to stderr and exit non-zero.
  - Clean the `nix eval` failure path: extract the actionable Nix `error:` lines
    from stderr and drop noise (the `warning: Git tree ... is dirty` line, etc.),
    so the user sees the real cause, not Nix's internal flake-path verbiage.
  - Confirm the four error classes already carry identity and surface cleanly:
    Nix eval, schema/IR validation (IngestIR names the resource/edge/path),
    provider gRPC diagnostics (summary+detail), and state-lock (names the path).
    Add a short, consistent prefix per class where missing.
- Docs:
  - Rewrite `README.md` from a seed-kit description into a real project README:
    what nixform is, the architecture in one paragraph, how to run the demo
    (build fakes, `nixform apply`/`destroy`, `nixform-gen`), and the test layout.
  - Add `docs/GETTING-STARTED.md`: a hands-on walkthrough using the fake
    providers (apply the headline topology, inspect state, destroy), runnable
    offline.
  - Add a short "stable contracts" note pointing at the frozen interfaces:
    `docs/IR-CONTRACT.md` + `docs/ir-schema.json` (the IR) and the flake
    `nixform.plan` interface (ledger-injected IR).

## Non-goals
- New runtime behavior, providers, or commands — presentation and documentation
  only.
- Structured/JSON error output or i18n — plain actionable text suffices for the PoC.
- Generated API reference docs — hand-written getting-started is enough.

## Impact
- Changed: `cmd/nixform/main.go` (cobra silencing + error helper),
  `internal/phase/evaluator.go` (clean nix-eval error), `README.md`. New:
  `docs/GETTING-STARTED.md`. A CLI test asserting clean error output.
- Completes E4c/d and the milestone's polish. No contract or IR change.
