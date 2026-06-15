# Tasks: nix-lib-core

- [x] 1.1 `nix/lib/ref.nix`: the reference representation — `__ref` and
      `__derived` leaves, detection helpers, `derived`/`str` builders that capture
      `<id>.<attr>` inputs + a render closure, and `resolve ledger v` that
      replaces a ref/derived with its concrete value when inputs are present.
- [x] 1.2 `nix/lib/mkResource.nix`: `mkResource { provider, type, name, config }`
      -> attrset with `id`, `config`, `meta`, and `refAttr`/`refPath` producing
      `__ref` leaves to this resource's outputs.
- [x] 1.3 `nix/lib/toIR.nix`: `toIR { providers, resources, nixConsumers ?, ledger ? }`
      -> canonical IR; resolves against the ledger, encodes/cleans leaves (strips
      internal __render/__inputRefs), derives the edge list from `__ref` usage
      (derived leaves create no edge).
- [x] 1.4 `nix/lib/default.nix` + `nix/lib/minilib.nix`: assemble the public lib
      on builtins only (no <nixpkgs>, so it evaluates without the binary cache).
- [x] 1.5 `flake.nix` + `nix/example/`: `nixform.plan = ledger -> IR`; example is
      the headline topology (A -> derived -> B -> derived -> C + a both-providers
      consumer). `nix eval .#nixform.plan --apply 'p: p {phase=0;outputs={};}'`
      yields conforming IR.
- [x] 1.6 `nix/tests/properties.nix`: 5 properties (leaves well-formed, ids
      unique, edge endpoints exist, ref->edge / derived->no-edge, ledger resolves
      ref+derived). Conformance + phased-resolution covered by the runner.
- [x] 1.7 `tests/run-nix-tests.sh`: runs the property eval, pipes phase-0 toIR
      through `check.py validate`, and asserts a derived value resolves to
      concrete with a ledger. Exits non-zero on failure.
- [x] 1.8 `openspec validate nix-lib-core` passes; change-id recorded in beans
      epic E1.
