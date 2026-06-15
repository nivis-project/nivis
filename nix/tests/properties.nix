# Property tests for the nivis Nix library, run via `nix eval`. Returns
# { ok = true; } when all properties hold, or throws with the failing case.
#
# Properties checked over several hand-rolled small graphs (Nix has no quickcheck;
# we enumerate representative shapes that cover the contract's leaf kinds):
#   P1. Every IR config/consumer leaf is a plain value, a well-formed __ref, or a
#       __derived (no internal __render/__inputRefs leak into the wire shape).
#   P2. Resource ids are unique.
#   P3. Every edge endpoint is an existing resource id.
#   P4. A direct ref produces an edge; a derived value does not.
#   P5. Injecting a ledger resolves __ref and __derived leaves to concrete values.
let
  nivis = import ../lib { };
  inherit (nivis) mkResource mkProvider toIR str;
  lib = nivis.lib;

  # --- graphs under test ----------------------------------------------------
  A = mkResource {
    provider = "alpha";
    type = "alpha_token";
    name = "A";
    config = { };
  };
  B = mkResource {
    provider = "beta";
    type = "beta_record";
    name = "B";
    config = {
      from = str [ "rec-" (A.refAttr "value") ]; # derived
    };
  };
  C = mkResource {
    provider = "alpha";
    type = "alpha_token";
    name = "C";
    config = {
      direct = A.refAttr "id"; # direct ref -> edge
      computed = str [ (B.refAttr "endpoint") "::" (A.refAttr "value") ];
    };
  };
  providers = {
    alpha = { source = "p"; config = { }; };
    beta = { source = "p"; config = { }; };
  };

  ir0 = toIR {
    inherit providers;
    resources = [ A B C ];
  };
  irResolved = toIR {
    inherit providers;
    resources = [ A B C ];
    ledger = {
      phase = 2;
      outputs = {
        "alpha.alpha_token.A" = { id = "alpha-0"; value = "V"; };
        "beta.beta_record.B" = { endpoint = "E"; };
      };
    };
  };

  # --- leaf walker ----------------------------------------------------------
  isPlainScalar = v: builtins.isString v || builtins.isInt v || builtins.isBool v || v == null || builtins.isFloat v;

  wellFormedLeaf =
    v:
    if v ? __ref then
      (v.__ref ? resource) && (v.__ref ? path) && (builtins.attrNames v.__ref == [ "path" "resource" ])
    else if v ? __derived then
      # wire shape must be ONLY { inputs }, no internal fields leaked
      builtins.attrNames v.__derived == [ "inputs" ]
    else
      false;

  # every leaf in a tree satisfies isPlainScalar or wellFormedLeaf
  leavesOk =
    v:
    if builtins.isAttrs v then
      if (v ? __ref) || (v ? __derived) || (v ? __sensitiveRef) then
        wellFormedLeaf v
      else
        builtins.all leavesOk (builtins.attrValues v)
    else if builtins.isList v then
      builtins.all leavesOk v
    else
      isPlainScalar v;

  ids = map (r: r.id) ir0.resources;
  unique = l: l == lib.concatLists (map (x: [ x ]) (builtins.attrNames (builtins.listToAttrs (map (x: { name = x; value = 0; }) l))));

  # --- assertions -----------------------------------------------------------
  P1 = builtins.all (r: leavesOk r.config) ir0.resources && builtins.all (c: leavesOk c.value) ir0.nixConsumers;
  P2 = (builtins.length ids) == (builtins.length (builtins.attrNames (builtins.listToAttrs (map (i: { name = i; value = 0; }) ids))));
  P3 = builtins.all (e: (builtins.elem e.from ids) && (builtins.elem e.to ids)) ir0.edges;
  P4 =
    let
      edgeVias = map (e: "${e.from}->${e.to}:${e.via}") ir0.edges;
    in
    builtins.elem "alpha.alpha_token.A->alpha.alpha_token.C:direct" edgeVias
    && !(builtins.any (e: e.via == "from") ir0.edges) # derived produced no edge
    && !(builtins.any (e: e.via == "computed") ir0.edges);
  P5 =
    let
      c = (builtins.elemAt irResolved.resources 2).config;
      bFrom = (builtins.elemAt irResolved.resources 1).config.from;
    in
    (c.direct == "alpha-0") && (c.computed == "E::V") && (bFrom == "rec-V");

  # P6. Provider config gets the same resolve+encode passes as resource config:
  #   - a __derived in provider config resolves against the ledger to a concrete value,
  #   - its leaves are well-formed wire shape (no internal fields leaked),
  #   - a nested provider block (default_tags.tags.x) and a plain value pass through,
  #   - source is verbatim.
  irProv =
    toIR {
      providers = {
        # config holds a __derived built from A.value, plus a nested block + scalar.
        aws = mkProvider {
          source = "registry.opentofu.org/hashicorp/aws";
          config = {
            region = "eu-central-1";
            token = str [ "tok-" (A.refAttr "value") ]; # __derived -> resolves to "tok-V"
            default_tags = { tags = { managed-by = "nivis"; }; };
          };
        };
      };
      resources = [ A ];
      ledger = {
        phase = 1;
        outputs = { "alpha.alpha_token.A" = { value = "V"; }; };
      };
    };
  P6 =
    let
      pc = irProv.providers.aws;
    in
    (pc.source == "registry.opentofu.org/hashicorp/aws")
    && (pc.config.region == "eu-central-1")
    && (pc.config.token == "tok-V") # __derived resolved against the ledger
    && (pc.config.default_tags.tags.managed-by == "nivis")
    && (leavesOk pc.config); # encoded to wire shape, no leaked internals

  # P7. mkProvider validates source: present+string -> ok; absent/empty -> throw.
  P7 =
    let
      ok = (mkProvider { source = "x"; }).source == "x";
      caught = (builtins.tryEval (mkProvider { config = { }; })).success == false;
      caughtEmpty = (builtins.tryEval (mkProvider { source = ""; })).success == false;
    in
    ok && caught && caughtEmpty;

  checks = [
    { name = "P1 leaves well-formed"; ok = P1; }
    { name = "P2 ids unique"; ok = P2; }
    { name = "P3 edge endpoints exist"; ok = P3; }
    { name = "P4 ref->edge, derived->no-edge"; ok = P4; }
    { name = "P5 ledger resolves ref+derived"; ok = P5; }
    { name = "P6 provider config resolves+encodes"; ok = P6; }
    { name = "P7 mkProvider validates source"; ok = P7; }
  ];
  failures = builtins.filter (c: !c.ok) checks;
in
if failures == [ ] then
  { ok = true; checks = map (c: c.name) checks; }
else
  throw "property failures: ${lib.concatMapStringsSep ", " (c: c.name) failures}"
