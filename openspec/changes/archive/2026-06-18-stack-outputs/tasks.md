# Tasks: stack-outputs

## 1. Nix: declare outputs
- [x] 1.1 `nix/lib/toIR.nix`: accept an `outputs ? {}` arg; for each `name`, emit a
      nixConsumer `{ id = "output.<name>"; value = { value = <expr>; }; }` merged
      into the consumer list (resolved + encoded like any consumer). Optional;
      empty => none.
- [x] 1.2 `nix/tests/properties.nix`: a property that `outputs = { x = A.refAttr
      "v"; }` yields an `output.x` consumer carrying the ref, and resolves to the
      concrete value against a ledger.

## 2. IR contract
- [x] 2.1 `docs/IR-CONTRACT.md`: a note that `output.<name>` is a reserved
      nixConsumer id for declared stack outputs (value shape `{ value }`); no
      schema change (ordinary consumers).

## 3. Executor: resolve outputs
- [x] 3.1 `internal/phase`: `ResolveOutputs(ctx) (map[string]interface{}, error)`:
      seed the ledger from state, eval, ingest, resolve, collect `output.`
      consumers from the (resolved) IR, unwrap `{value}` -> the value, keyed by the
      name after the `output.` prefix.
- [x] 3.2 Unit test: a stubbed IR with `output.` consumers -> ResolveOutputs
      returns the unwrapped name->value map.

## 4. CLI: nivis output
- [x] 4.1 `cmd/nivis`: `output [name]` command. No name -> all as `name = value`
      lines (sorted); a name -> just that value (error if not declared). `--json`
      -> a JSON object (or the single value). Writes via cmd.OutOrStdout().
- [x] 4.2 Unit test: render all (plain), single name, --json, unknown-name error.

## 5. E2E (the required end-to-end)
- [x] 5.1 An e2e against the fake provider binaries: a config declaring an output =
      one resource attr AND an output composed across two resources (resolving
      across phases); apply to fixpoint; ResolveOutputs returns both concrete.
      Assert the composed value equals the value built from the two resolved attrs.
      (Use the real-Nix flake e2e style, or the StubEvaluator + fakes if a flake
      output attr is awkward; prefer the real flake so it's a true e2e.)
- [x] 5.2 The example `nix/example/aws.nix` (or the e2e flake attr) declares a
      couple of outputs so the path is exercised by a real flake eval.

## 6. Docs (docs-coverage gate: new section)
- [x] 6.1 getting-started: an "Outputs" section (declare with `outputs`, read with
      `nivis output [--json]`). No em dashes.

## 7. Gate
- [x] 7.1 `gofmt`, `go build ./...`, `go test ./...` green.
- [x] 7.2 `bash tests/run-nix-tests.sh` green; `bash tests/check-docs-ssot.sh` green.
- [x] 7.3 `openspec validate stack-outputs --strict`; archive; close beans-h9ws.
