# toIR serializes a resource set to the canonical JSON IR (docs/IR-CONTRACT.md /
# docs/ir-schema.json). It resolves leaves against the outputs ledger first, then
# encodes any remaining __ref/__derived placeholders and derives the edge list.
{ lib, ref }:
let
  inherit (ref) isRef isDerived isSensitiveRef isBuild resolve;

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
    else if isRef v || isSensitiveRef v || isBuild v then
      v # __ref/__sensitiveRef/__build are already in wire shape
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
# toIR :: { providers, resources, dataSources ? [], nixConsumers ? [], outputs ? {}, backend ? null, ledger ? {} } -> IR attrset
{
  providers,
  resources,
  dataSources ? [ ],
  nixConsumers ? [ ],
  # Declared stack outputs: name -> value expression. Each becomes a reserved
  # nixConsumer `output.<name>` with value { value = <expr>; }, so it rides the
  # existing consumer resolution and is read out by `nivis output`.
  outputs ? { },
  # Optional remote-state backend declaration, e.g.
  #   { type = "s3"; bucket = "..."; key = "..."; region = "..."; }
  # Static config: plain values only (no refs/derived), since the executor must
  # know where state lives before it evaluates anything. Emitted only when set;
  # absent => the local file store. Credentials are NEVER here (AWS chain).
  backend ? null,
  ledger ? { outputs = { }; },
}:
let
  # Stack outputs are reserved consumers (id "output.<name>"), merged with any
  # explicit nixConsumers before resolution.
  outputConsumers = lib.mapAttrsToList (name: expr: {
    id = "output.${name}";
    value = { value = expr; };
  }) outputs;
  allConsumers = nixConsumers ++ outputConsumers;

  # resolve every resource/datasource config and consumer value against the ledger first.
  resolvedResources = map (r: r // { config = resolve ledger r.config; }) resources;
  resolvedDataSources = map (d: d // { config = resolve ledger d.config; }) dataSources;
  resolvedConsumers = map (c: c // { value = resolve ledger c.value; }) allConsumers;

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

  # Datasources mirror resources but carry no meta (read, never applied).
  irDataSources = map (d: {
    inherit (d) id provider type name;
    config = encode d.config;
  }) resolvedDataSources;

  # Edges come from BOTH resources and datasources: a ref to a datasource id
  # (or a datasource config referencing another node) is an ordinary edge.
  edges =
    lib.concatMap (r: collectEdges r.id r.config) resolvedResources
    ++ lib.concatMap (d: collectEdges d.id d.config) resolvedDataSources;

  irConsumers = map (c: {
    inherit (c) id;
    value = encode c.value;
  }) resolvedConsumers;
in
{
  schemaVersion = 1;
  providers = irProviders;
  resources = irResources;
  dataSources = irDataSources;
  inherit edges;
  nixConsumers = irConsumers;
}
# The backend is emitted only when declared (omitted => local file store). It is
# static config, passed through verbatim (no resolve/encode: it must not contain
# refs or unknowns).
// lib.optionalAttrs (backend != null) { inherit backend; }
