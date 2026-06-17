# The public nivis Nix library. Pure: depends on builtins + a self-contained
# minilib (no <nixpkgs>, so it evaluates without the binary cache).
#
# Usage:
#   let nivis = import ./nix/lib { };
#   in nivis.toIR { providers = {...}; resources = [ (nivis.mkResource {...}) ]; }
{
  lib ? import ./minilib.nix,
}:
let
  ref = import ./ref.nix { inherit lib; };
  mkResource = import ./mkResource.nix { inherit lib ref; };
  mkProvider = import ./mkProvider.nix { inherit lib; };
  toIR = import ./toIR.nix { inherit lib ref; };
  expand = import ./expand.nix { inherit lib mkResource; };
  modules = import ./modules.nix { inherit lib toIR; };
  vars = import ./vars.nix { inherit lib; };
in
{
  inherit
    lib
    ref
    mkResource
    mkProvider
    toIR
    ;

  # Configuration variables (typed, with defaults; resolved from ledger.vars).
  inherit (vars) mkVars;

  # Expansion (count / for_each).
  inherit (expand) mkResources;

  # Module composition.
  inherit (modules) evalModules toModuleIR;

  # Convenience re-exports of the most-used ref helpers.
  inherit (ref)
    mkRef
    derived
    str
    drv
    drvFile
    resolve
    isRef
    isDerived
    isBuild
    ;
}
