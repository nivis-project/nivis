# count / for_each expansion. Expansion happens in NIX (IR contract): mkResources
# produces concrete, already-multiplied resource instances with deterministic ids
# (<name>__<index> for count, <name>__<key> for for_each). The executor never
# sees count/for_each — only the expanded instances.
{ lib, mkResource }:
let
  # mkResources expands a base spec into a LIST of resource instances.
  #
  # Spec:
  #   { provider, type, name, count ? null, forEach ? null, config, meta ? null }
  # where:
  #   - exactly one of `count` (int) or `forEach` (attrset) is set, OR neither
  #     (then it behaves like a single mkResource).
  #   - `config` may be a plain attrset, or a function:
  #       count:   idx -> config attrset
  #       forEach: key: value: config attrset   (curried)
  #   - `meta` similarly may be a value or a function of the key/index.
  mkResources =
    spec:
    let
      base = {
        inherit (spec) provider type;
        meta = spec.meta or null;
      };

      # resolve a possibly-function field for an instance.
      callIf = f: args: if builtins.isFunction f then applyAll f args else f;
      applyAll = f: args: builtins.foldl' (g: a: g a) f args;

      single =
        suffix: cfgArgs:
        mkResource (
          base
          // {
            name = "${spec.name}__${suffix}";
            config = callIf spec.config cfgArgs;
            meta = callIf (spec.meta or null) cfgArgs;
          }
        );
    in
    if (spec.count or null) != null then
      builtins.genList (i: single (toString i) [ i ]) spec.count
    else if (spec.forEach or null) != null then
      lib.mapAttrsToList (key: value: single key [ key value ]) spec.forEach
    else
      # neither count nor forEach: a single un-suffixed resource.
      [
        (mkResource {
          inherit (spec) provider type name;
          config = if builtins.isFunction spec.config then spec.config else spec.config;
          meta = spec.meta or null;
        })
      ];
in
{
  inherit mkResources;
}
