# Proposal: stack-outputs

## Why
A user can read a single resource's attributes with `nivis state show <id>`, but
there is no way to **surface named values out of a run** (the Terraform
`output "x" {}` equivalent): a human-readable "here are my stack's results" or a
machine-readable form a CI step or another stack can consume. This is A7 of "Road
to v1" (beans-h9ws), filed while reviewing datasources: it is the orthogonal
*other* meaning of "outputs" (values out of the run), distinct from a datasource's
attributes flowing back in. The resolution machinery already exists: the IR's
`nixConsumers` are values Nix computes from resource/datasource outputs and that
are fully concrete at fixpoint (`res.LastIR.Consumers`). This adds the user-facing
declaration + read surface on top, reusing that plumbing.

## What changes
- **Declare outputs in Nix:** `toIR { ...; outputs = { ip = instance.refAttr
  "public_ip"; url = str [ "http://" (instance.refAttr "public_ip") ]; }; }`. Each
  named output becomes a reserved nixConsumer with id `output.<name>` and value
  `{ value = <expr>; }`. This reuses the consumer resolution (no new IR node type,
  no new resolution path); it is a naming convention plus a toIR arg.
- **`nivis output [name]`** prints the resolved outputs after apply: all of them
  (human-readable `name = value` lines), or just `[name]`. It resolves them by
  seeding the ledger from current state and re-evaluating read-only (the same
  pattern `plan` uses), so it works as a standalone command after apply with no
  always-written artifact.
- **`nivis output --json`** prints a JSON object `{ "<name>": <value> }` for a CI
  step or another stack to consume.
- **Sensitive outputs** follow the existing sensitive-value handling: a sensitive
  value is surfaced via the restricted channel, never written world-readable.
- **The executor** gains a `ResolveOutputs` that returns the resolved
  `output.<name>` map (seed-from-state + eval + read the `output.` consumers).

## Decisions (settled with the maintainer)
- **`outputs` arg to `toIR`**, not a separate `nivis.outputs` flake attr (reuses
  the call the examples already make and the same ledger/eval; minimal surface).
- **`--json` on demand**, not an always-written `outputs.json` (explicit, composes
  with pipes, no second artifact to keep fresh).
- Outputs are a **named layer over `nixConsumers`** (reserved `output.` ids), not a
  new IR node type.

## Non-goals
- Remote/shared output state (pairs with B1 remote state).
- Cross-stack auto-wiring / a registry of other stacks' outputs (later).
- Output *change* tracking in the plan (outputs are read post-apply, not planned).

## Impact
- Nix: `nix/lib/toIR.nix` accepts `outputs` and emits one `output.<name>` consumer
  each; `nix/lib` may expose a tiny helper. The example `nix/example/aws.nix` (and
  the e2e flake) declare outputs so the e2e exercises them.
- IR: `docs/IR-CONTRACT.md` documents the reserved `output.` consumer-id convention
  (a small contract note; no schema change, since these are ordinary consumers).
- Go: `internal/phase` `ResolveOutputs` (seed-from-state, eval, collect `output.`
  consumers, unwrap `{value}`); `cmd/nivis` `output` command with `--json`.
- Tests: nix property (outputs become `output.` consumers); Go (ResolveOutputs
  collects + unwraps); **an e2e against the in-repo fake providers**: apply a
  graph that declares outputs derived from BOTH providers across phases, then
  `nivis output` / `ResolveOutputs` returns the concrete values (human + json).

Docs impact: new section; an "Outputs" section in getting-started (declare with
`outputs`, read with `nivis output [--json]`) plus the IR-CONTRACT note on the
reserved id. No new standalone document: outputs are part of the existing
plan/apply/state flow (per docs/DOCS-GATE.md).
