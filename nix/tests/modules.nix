# Property tests for expansion (count/for_each) and the module system. Run via
# `nix eval`. Returns { ok = true; } or throws naming the failed check.
let
  nf = import ../lib { };
  lib = nf.lib;
  inherit (nf) mkResource mkResources toModuleIR;

  ids = rs: map (r: r.id) rs;
  has = xs: x: builtins.elem x xs;

  # --- expansion -------------------------------------------------------------
  countInstances = mkResources {
    provider = "alpha";
    type = "alpha_token";
    name = "web";
    count = 3;
    config = i: { label = "web-${toString i}"; };
  };
  feInstances = mkResources {
    provider = "alpha";
    type = "alpha_token";
    name = "tok";
    forEach = {
      a = "A";
      b = "B";
    };
    config = key: value: { label = "${key}-${value}"; };
  };

  # a resource referencing an expanded instance -> ordinary __ref to <base>__a
  consumer = mkResource {
    provider = "alpha";
    type = "alpha_token";
    name = "c";
    config = {
      x = (builtins.head feInstances).refAttr "value"; # the __a instance
    };
  };

  # --- modules ---------------------------------------------------------------
  modA = { tf, ... }: {
    providers.alpha = {
      source = "p";
      config = { };
    };
    resources = [ (mkResource { provider = "alpha"; type = "alpha_token"; name = "A"; config = { }; }) ];
  };
  modB =
    { tf, ... }:
    {
      resources = [
        (mkResource {
          provider = "alpha";
          type = "alpha_token";
          name = "B";
          config = {
            up = tf."alpha.alpha_token.A".refAttr "value";
          };
        })
      ];
      nixConsumers = [
        {
          id = "out";
          value = {
            aVal = tf."alpha.alpha_token.A".refAttr "value";
          };
        }
      ];
    };
  modIR = toModuleIR { modules = [ modA modB ]; };

  # duplicate-id detection: tryEval must report failure.
  dupAttempt =
    let
      mk = name: { ... }: { resources = [ (mkResource { provider = "alpha"; type = "alpha_token"; inherit name; config = { }; }) ]; };
    in
    builtins.tryEval (builtins.length (toModuleIR { modules = [ (mk "DUP") (mk "DUP") ]; }).resources);

  # --- checks ----------------------------------------------------------------
  P1 = ids countInstances == [
    "alpha.alpha_token.web__0"
    "alpha.alpha_token.web__1"
    "alpha.alpha_token.web__2"
  ];
  P2 = (has (ids feInstances) "alpha.alpha_token.tok__a") && (has (ids feInstances) "alpha.alpha_token.tok__b");
  P3 = consumer.config.x.__ref.resource == "alpha.alpha_token.tok__a";
  P4 = (has (ids modIR.resources) "alpha.alpha_token.A") && (has (ids modIR.resources) "alpha.alpha_token.B");
  # cross-module ref + derived edge present
  P5 =
    let
      bUp = (builtins.elemAt modIR.resources 1).config.up;
    in
    bUp.__ref.resource == "alpha.alpha_token.A"
    && builtins.any (e: e.from == "alpha.alpha_token.A" && e.to == "alpha.alpha_token.B") modIR.edges;
  # consumer reads A across module boundary
  P6 = (builtins.head modIR.nixConsumers).value.aVal.__ref.resource == "alpha.alpha_token.A";
  # duplicate id is rejected (tryEval .success == false)
  P7 = dupAttempt.success == false;

  checks = [
    { name = "P1 count expands to N ids"; ok = P1; }
    { name = "P2 for_each expands to keyed ids"; ok = P2; }
    { name = "P3 ref into instance is <base>__key"; ok = P3; }
    { name = "P4 modules merge to one graph"; ok = P4; }
    { name = "P5 cross-module ref + edge"; ok = P5; }
    { name = "P6 consumer reads across modules"; ok = P6; }
    { name = "P7 duplicate id rejected"; ok = P7; }
  ];
  failures = builtins.filter (c: !c.ok) checks;
in
if failures == [ ] then
  { ok = true; checks = map (c: c.name) checks; }
else
  throw "module/expansion property failures: ${lib.concatMapStringsSep ", " (c: c.name) failures}"
