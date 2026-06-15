# toIR serializes a resource set to the canonical JSON IR (docs/IR-CONTRACT.md /
# docs/ir-schema.json). It resolves leaves against the outputs ledger first, then
# encodes any remaining __ref/__derived placeholders and derives the edge list.
{ lib, ref }:
let
  inherit (ref) isRef isDerived isSensitiveRef resolve;

  # cleanLeaf strips internal-only fields from a __derived leaf so the IR carries
  # only { inputs } as the contract specifies (the render closure and input refs
  # are eval-time machinery, not part of the wire format).
  cleanLeaf = v: {
    __derived = {
      inherit (v.__derived) inputs;
    };
  };

  # encode walks a (already ledger-resolved) value tree, normalizing markers.
  encode =
    v:
    if isDerived v then
      cleanLeaf v
    else if isRef v || isSensitiveRef v then
      v # __ref/__sensitiveRef are already in wire shape
    else if builtins.isAttrs v then
      builtins.mapAttrs (_: encode) v
    else if builtins.isList v then
      map encode v
    else
      v;

  # collectEdges walks a resource's config and returns edges {from,to,via} for
  # every __ref/__sensitiveRef found (via = the top-level config attr carrying
  # the dependency). Derived leaves do not create executor edges.
  collectEdges =
    toId: config:
    lib.concatLists (
      lib.mapAttrsToList (
        attrName: value:
        let
          refsIn = findRefs value;
        in
        map (target: {
          from = target;
          to = toId;
          via = attrName;
        }) refsIn
      ) config
    );

  # findRefs :: value -> list of target resource ids referenced anywhere within.
  findRefs =
    v:
    if isRef v then
      [ v.__ref.resource ]
    else if isSensitiveRef v then
      [ v.__sensitiveRef.resource ]
    else if isDerived v then
      [ ] # derived inputs are *->Nix, not executor edges
    else if builtins.isAttrs v then
      lib.concatLists (lib.mapAttrsToList (_: findRefs) v)
    else if builtins.isList v then
      lib.concatLists (map findRefs v)
    else
      [ ];
in
# toIR :: { providers, resources, nixConsumers ? [], ledger ? {} } -> IR attrset
{
  providers,
  resources,
  nixConsumers ? [ ],
  ledger ? { outputs = { }; },
}:
let
  # resolve every resource config and consumer value against the ledger first.
  resolvedResources = map (r: r // { config = resolve ledger r.config; }) resources;
  resolvedConsumers = map (c: c // { value = resolve ledger c.value; }) nixConsumers;

  # Provider config gets the SAME two passes as resource config: resolve against
  # the ledger (so a __ref/__derived in provider config resolves each phase) then
  # encode to wire shape. `source` passes through verbatim. Plain values are a
  # no-op. Accepts the bare { source, config } shape and mkProvider's output alike.
  irProviders = builtins.mapAttrs (
    _: p: {
      inherit (p) source;
      config = encode (resolve ledger (p.config or { }));
    }
  ) providers;

  irResources = map (r: {
    inherit (r) id provider type name;
    config = encode r.config;
  } // lib.optionalAttrs (r ? meta && r.meta != null) { inherit (r) meta; }) resolvedResources;

  edges = lib.concatMap (r: collectEdges r.id r.config) resolvedResources;

  irConsumers = map (c: {
    inherit (c) id;
    value = encode c.value;
  }) resolvedConsumers;
in
{
  schemaVersion = 1;
  providers = irProviders;
  resources = irResources;
  inherit edges;
  nixConsumers = irConsumers;
}
