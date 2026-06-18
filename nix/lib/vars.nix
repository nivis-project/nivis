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

  # CLI / NIVIS_VAR_* values always arrive as STRINGS, but a variable may be
  # declared int or bool. coerce parses a string to the declared scalar type so
  # `--var replicas=5` satisfies an int var; a value already of the right type (or
  # a typed JSON --var-file value) passes through. An unparseable string throws a
  # clear, named error. `str`/`any` keep the value as-is.
  coerce =
    name: type: value:
    if type == "int" && builtins.isString value then
      (
        # only fromJSON-parse a string that LOOKS like an integer; fromJSON aborts
        # (uncatchable) on non-JSON, so guard with a digit check first to give a
        # clean, named error instead of a raw parse trace.
        if builtins.match "-?[0-9]+" value != null then
          builtins.fromJSON value
        else
          throw "nivis.mkVars: variable '${name}' expects type 'int', got '${value}'"
      )
    else if type == "bool" && builtins.isString value then
      (
        if value == "true" then
          true
        else if value == "false" then
          false
        else
          throw "nivis.mkVars: variable '${name}' expects type 'bool', got '${value}' (use true/false)"
      )
    else
      value;

  resolveOne =
    injected: name: decl:
    let
      type = decl.type or "any";
      checker =
        typeOk.${type} or (throw "nivis.mkVars: variable '${name}' has unknown type '${type}' (expected one of str, int, bool, any)");
      isSet = injected ? ${name};
      hasDefault = decl ? default;
      value = coerce name type injected.${name};
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
