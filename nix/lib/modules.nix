# A minimal evalModules-style composition (no nixpkgs, CLAUDE.md §6). Each module
# is `{ config, tf, lib, ... }: { resources ? []; providers ? {}; nixConsumers ? []; }`.
# Modules compose into one flat graph; a module may reference resources declared
# in any module via `tf` (an id -> resource map), resolved by a lazy fixpoint.
{ lib, toIR }:
let
  # evalModules :: { modules; specialArgs ? {} } -> merged graph
  #   { resources = [..]; providers = {..}; nixConsumers = [..]; tf = { <id> = res; }; }
  evalModules =
    {
      modules,
      specialArgs ? { },
    }:
    let
      # Lazy fixpoint: each module is called with `tf` = the merged id->resource
      # map of ALL modules. Nix laziness lets module B read module A's resource
      # (and vice versa) as long as the *values used* don't form a strict cycle
      # — refAttr only needs ids, which are computed from coordinates, not config.
      merged = rec {
        evaluated = map (
          m:
          m (
            specialArgs
            // {
              inherit lib tf;
              config = specialArgs.config or { };
            }
          )
        ) modules;

        rawResources = lib.concatLists (map (e: e.resources or [ ]) evaluated);
        nixConsumers = lib.concatLists (map (e: e.nixConsumers or [ ]) evaluated);
        providers = builtins.foldl' (acc: e: acc // (e.providers or { })) { } evaluated;

        # tf: id -> resource, with a duplicate-id check (throws on conflict).
        tf = buildTf rawResources;

        # resources forces the uniqueness check (via tf) by using its size as a
        # seq, so a duplicate id is rejected even when only `resources` is read
        # (not just `tf`). `builtins.seq` evaluates tf to WHNF first.
        resources = builtins.seq (builtins.length (builtins.attrNames tf)) rawResources;
      };
    in
    {
      inherit (merged)
        resources
        providers
        nixConsumers
        tf
        ;
    };

  # buildTf folds the resource list into an id->resource map, throwing on a
  # duplicate id (named).
  buildTf =
    resources:
    builtins.foldl' (
      acc: r:
      if acc ? ${r.id} then
        throw "nixform: duplicate resource id '${r.id}' across modules"
      else
        acc // { ${r.id} = r; }
    ) { } resources;

  # toModuleIR :: { modules; specialArgs ? {} } -> IR (optionally ledger-resolved)
  toModuleIR =
    args@{
      modules,
      specialArgs ? { },
      ledger ? { outputs = { }; },
    }:
    let
      g = evalModules { inherit modules specialArgs; };
    in
    toIR {
      inherit (g) providers resources nixConsumers;
      inherit ledger;
    };
in
{
  inherit evalModules toModuleIR;
}
