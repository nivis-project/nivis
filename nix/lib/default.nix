# The public nixform Nix library. Pure: depends on builtins + a self-contained
# minilib (no <nixpkgs>, so it evaluates without the binary cache).
#
# Usage:
#   let nixform = import ./nix/lib { };
#   in nixform.toIR { providers = {...}; resources = [ (nixform.mkResource {...}) ]; }
{
  lib ? import ./minilib.nix,
}:
let
  ref = import ./ref.nix { inherit lib; };
  mkResource = import ./mkResource.nix { inherit lib ref; };
  toIR = import ./toIR.nix { inherit lib ref; };
  expand = import ./expand.nix { inherit lib mkResource; };
  modules = import ./modules.nix { inherit lib toIR; };
in
{
  inherit
    lib
    ref
    mkResource
    toIR
    ;

  # Expansion (count / for_each).
  inherit (expand) mkResources;

  # Module composition.
  inherit (modules) evalModules toModuleIR;

  # Convenience re-exports of the most-used ref helpers.
  inherit (ref) mkRef derived str resolve isRef isDerived;
}
