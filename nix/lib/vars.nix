# mkVars: declare typed configuration variables and resolve them against the
# values the executor injects in `ledger.vars`. Pure (builtins only, no IO, no
# environment reads): it validates data already passed in.
#
#   vars = nivis.mkVars {
#     region = { type = "str"; default = "eu-central-1"; };
#     suffix = { type = "str"; };   # no default -> required
#   } (ledger.vars or { });
#   # then read vars.region, vars.suffix in the plan.
#
# Resolution per declared variable:
#   - set (present in injected)      -> the injected value (type-checked)
#   - unset, has a default           -> the default
#   - unset, no default              -> required: throw, naming the variable
# Injected values for UNdeclared names are ignored (only declared vars returned).
{ lib }:
let
  # type name -> predicate. "any" accepts anything.
  typeOk = {
    str = builtins.isString;
    int = builtins.isInt;
    bool = builtins.isBool;
    any = _: true;
  };

  resolveOne =
    injected: name: decl:
    let
      type = decl.type or "any";
      checker =
        typeOk.${type} or (throw "nivis.mkVars: variable '${name}' has unknown type '${type}' (expected one of str, int, bool, any)");
      isSet = injected ? ${name};
      hasDefault = decl ? default;
      value = injected.${name};
    in
    if isSet then
      (
        if checker value then
          value
        else
          throw "nivis.mkVars: variable '${name}' expects type '${type}', got ${builtins.typeOf value}"
      )
    else if hasDefault then
      decl.default
    else
      throw "nivis.mkVars: required variable '${name}' is not set (declare a default or pass --var ${name}=...)";

  mkVars =
    decls: injected:
    builtins.mapAttrs (name: decl: resolveOne injected name decl) decls;
in
{
  inherit mkVars;
}
