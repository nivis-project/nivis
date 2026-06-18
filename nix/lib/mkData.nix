# mkData builds a datasource value: existing infrastructure to READ, not create.
# It mirrors mkResource (stable id + refAttr/refPath output access) but has no
# meta/lifecycle, and its id is namespaced `data.<provider>.<type>.<name>` so it
# never collides with a resource id. A resource (or another datasource) wires a
# datasource output in exactly as it references a resource: `d.refAttr "id"`.
{ lib, ref }:
let
  inherit (ref) mkRef;
in
# mkData :: { provider, type, name, config ? {} } -> datasource
#
# The returned attrset exposes:
#   id, provider, type, name, config  - the datasource data
#   isData = true                     - marks it for toIR's dataSources array
#   refAttr <attr>  - a __ref leaf to this datasource's output <attr>
#   refPath <path>  - a __ref leaf to a nested output path
{
  provider,
  type,
  name,
  config ? { },
}:
let
  id = "data.${provider}.${type}.${name}";
in
{
  inherit
    id
    provider
    type
    name
    config
    ;
  isData = true;
  refAttr = attr: mkRef id [ attr ];
  refPath = path: mkRef id path;
}
