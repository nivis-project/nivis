# mkResource builds a resource value with a stable identity and an output-access
# mechanism: `r.refAttr "x"` yields a __ref to this resource's output `x`, so
# dependency edges between resources are implicit in how authors wire configs.
{ lib, ref }:
let
  inherit (ref) mkRef;
in
# mkResource :: { provider, type, name, config ? {}, meta ? null } -> resource
#
# The returned attrset exposes:
#   id, provider, type, name, config, meta  - the resource data
#   refAttr <attr>  - a __ref leaf to this resource's output <attr>
#   refPath <path>  - a __ref leaf to a nested output path (list of keys/indices)
#
# We cannot enumerate a provider's computed outputs without its schema, so output
# references are explicit (refAttr/refPath) rather than a fixed `outputs` set.
{
  provider,
  type,
  name,
  config ? { },
  meta ? null,
}:
let
  id = "${provider}.${type}.${name}";
in
{
  inherit
    id
    provider
    type
    name
    config
    meta
    ;
  refAttr = attr: mkRef id [ attr ];
  refPath = path: mkRef id path;
}
