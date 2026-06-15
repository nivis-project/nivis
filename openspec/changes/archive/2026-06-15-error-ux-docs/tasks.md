# Tasks: error-ux-docs

- [x] 1.1 `cmd/nixform/main.go`: `SilenceErrors` + a `PersistentPreRun` that sets
      `SilenceUsage` only after flags parse, so runtime (RunE) errors print a
      clean `error:` line with no usage dump, while flag misuse still shows usage.
- [x] 1.2 `internal/phase/evaluator.go`: `cleanNixStderr` keeps the Nix `error:`
      lines + indented context and drops warnings (the "Git tree is dirty"
      noise); the failure message names the failing flake attr.
- [x] 1.3 Audited the four classes carry identity: IR validation (IngestIR names
      resource/edge/path), provider diagnostics (summary+detail), state path,
      phase stuck-resource + awaited inputs. Consistent prefixes confirmed.
- [x] 1.4 `cmd/nixform/main_test.go`: runtime error prints `error:` and NO usage
      block and omits the dirty-tree warning; flag misuse DOES show usage.
- [x] 1.5 Rewrote `README.md`: what nixform is, one-paragraph architecture, the
      offline demo (build fakes, plan/apply/state/refresh/destroy, nixform-gen),
      layout, stable contracts, testing.
- [x] 1.6 `docs/GETTING-STARTED.md`: offline hands-on walkthrough with real,
      verified command output (apply across 3 phases, state show the both-
      providers derived value, destroy in reverse order, gen a constructor).
- [x] 1.7 "Stable contracts" section in the README pointing at docs/IR-CONTRACT.md
      + docs/ir-schema.json (the IR) and the flake nixform.plan interface.
- [x] 1.8 `go test ./...` + `go vet ./...` pass; nix tests + IR conformance green.
- [x] 1.9 `openspec validate error-ux-docs` passes; change-id in beans E4c/d.
