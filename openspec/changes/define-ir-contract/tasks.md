# Tasks: define-ir-contract

- [ ] 1.1 Author `docs/IR-CONTRACT.md` top-level shape (providers, resources,
      edges, nixConsumers) with `schemaVersion: 1`.
- [ ] 1.2 Specify `__ref` encoding: `{resource, path}` where `path` mixes string
      keys and integer indices; cover nested attrs, list/set indices, map keys,
      and refs into expanded `for_each`/`count` instances.
- [ ] 1.3 Specify `__derived` encoding for Nix-computed leaves and the rule that
      they are *->Nix (resolved only by re-eval, never in-executor).
- [ ] 1.4 Specify the `__ref` -> tfprotov6 unknown-value mapping the executor
      applies at `PlanResourceChange`.
- [ ] 1.5 Specify `for_each`/`count` expansion timing: expansion in Nix; IR
      carries concrete instances with `<base>__<key>` ids; executor never sees
      count/for_each.
- [ ] 1.6 Specify sensitive-value handling: never emitted into `nix eval` JSON or
      the store; lives only in the 0600 outputs ledger; injected to re-eval via a
      private 0600 channel referenced as `__sensitiveRef`.
- [ ] 1.7 Specify the outputs-ledger injection format (`phase`, `outputs`) read by
      the flake `plan` argument on each phase.
- [ ] 1.8 Define conformance requirements: Nix property test obligations for
      `toIR`; Go `IngestIR` validation obligations (malformed-IR errors name the
      offending resource/path).
- [ ] 1.9 `openspec validate define-ir-contract` passes; cross-link the beans
      epic E1.5 with this change-id.
