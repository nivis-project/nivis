# Tasks: nix-lib-modules

- [x] 1.1 minilib: confirmed `concatLists`/`mapAttrsToList` already present;
      `genList`/`foldl'`/`mapAttrs` used as builtins. No additions needed.
- [x] 1.2 `nix/lib/expand.nix`: `mkResources` for `count` (ids `<name>__<index>`)
      and `forEach` (ids `<name>__<key>`, via builtins.mapAttrs); config/meta may
      be functions of the index or key/value. Never emits count/forEach.
- [x] 1.3 `nix/lib/mkResource.nix`: dependsOn/lifecycle flow into IR meta (toIR);
      count/forEach passed to mkResource are rejected by strict formals (must use
      mkResources).
- [x] 1.4 `nix/lib/modules.nix`: `evalModules` threads `{ config, tf, lib, ... }`,
      merges resources/providers/nixConsumers, exposes `tf` (id->resource) for
      cross-module refs via a lazy fixpoint, and throws on a duplicate id (named;
      forced eagerly so reading `resources` triggers the check). `toModuleIR` =
      evalModules + toIR.
- [x] 1.5 `nix/lib/default.nix`: exports `mkResources`, `evalModules`, `toModuleIR`.
- [x] 1.6 `nix/tests/modules.nix`: 7 properties — count/for_each ids, ref into an
      instance, modules merge, cross-module ref + edge, consumer across modules,
      duplicate-id rejected (tryEval). All green.
- [x] 1.7 `tests/run-nix-tests.sh`: added the module/expansion property eval and a
      conformance check that an expanded + module-composed IR validates.
- [x] 1.8 `openspec validate nix-lib-modules` passes; E1 complete.
