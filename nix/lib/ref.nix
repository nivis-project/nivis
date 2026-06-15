# The reference system: how a Nix author refers to apply-time provider outputs.
#
# A __ref leaf is a direct reference to a resource output attribute. A __derived
# leaf is a value COMPUTED from one or more outputs (Nix cannot produce its
# concrete value until those outputs are known and Nix is re-evaluated — DESIGN
# D3). Both are plain attrsets carrying a reserved marker key, so they serialize
# straight into the IR and survive a phase unresolved.
{ lib }:
rec {
  # mkRef :: id -> path(list) -> __ref leaf
  mkRef = resource: path: { __ref = { inherit resource path; }; };

  # isRef / isDerived / isMarker :: any -> bool
  isRef = v: builtins.isAttrs v && v ? __ref;
  isDerived = v: builtins.isAttrs v && v ? __derived;
  isSensitiveRef = v: builtins.isAttrs v && v ? __sensitiveRef;
  isBuild = v: builtins.isAttrs v && v ? __build;
  isMarker = v: isRef v || isDerived v || isSensitiveRef v;

  # buildLeaf :: store-path-string -> __build leaf. The executor realises this
  # path (builds the derivation) before apply, then substitutes it into the
  # provider config. Unlike __ref/__derived, a __build leaf is a KNOWN value (its
  # path exists at evaluation); only the file needs building, so it passes through
  # `resolve` unchanged.
  buildLeaf = path: { __build = { inherit path; }; };

  # drv :: derivation -> __build leaf. Marks a config value as the output of a Nix
  # build (e.g. aws_s3_object.source = drv image). If the derivation has
  # passthru.filePath (an artifact inside its output, like a NixOS image's .vhd),
  # that file is used; otherwise the output root. For an explicit sub-path use
  # `drvFile d "rel/path"`.
  drv = d: buildLeaf (if d ? passthru && d.passthru ? filePath then "${d}/${d.passthru.filePath}" else "${d}");

  # drvFile :: derivation -> relative-path -> __build leaf for "${d}/<path>".
  drvFile = d: file: buildLeaf "${d}/${file}";

  # inputsOf :: marker -> list of "<id>.<attr>" input keys it depends on.
  # For a __ref, that is the single "<resource>.<path-dotted>"; for a __derived,
  # the recorded inputs; empty for a plain value.
  inputsOf =
    v:
    if isRef v then
      [ (dotted v.__ref.resource v.__ref.path) ]
    else if isSensitiveRef v then
      [ (dotted v.__sensitiveRef.resource v.__sensitiveRef.path) ]
    else if isDerived v then
      v.__derived.inputs
    else
      [ ];

  dotted = resource: path: resource + "." + lib.concatMapStringsSep "." toString path;

  # derived :: { inputs, render } -> __derived leaf.
  # `inputs` is the list of refs/markers this value is computed from; `render` is
  # a function from the resolved input values (in order) to the concrete value.
  # The render function is stored so `resolve` can produce the concrete value
  # once every input is known. Until then the leaf carries only the input keys
  # (what the IR records) — render is dropped at serialization time.
  derived =
    {
      inputs,
      render,
    }:
    {
      __derived = {
        inputs = lib.concatMap inputsOf inputs;
        # __render is an internal field used only by resolve; toIR strips it.
        __render = render;
        __inputRefs = inputs;
      };
    };

  # str :: list -> a __derived (or plain string) concatenation.
  # Each part is either a plain string or a ref/marker. If no part is a marker,
  # the result is a plain concatenated string (fully known). Otherwise it is a
  # __derived whose render concatenates the resolved parts.
  str =
    parts:
    if !(builtins.any isMarker parts) then
      lib.concatStrings parts
    else
      derived {
        inputs = builtins.filter isMarker parts;
        render =
          resolved:
          let
            # rebuild the concatenation, substituting resolved values for markers
            # in order. `resolved` is the list of resolved marker values.
            go =
              acc: remaining: parts':
              if parts' == [ ] then
                acc
              else
                let
                  p = builtins.head parts';
                  rest = builtins.tail parts';
                in
                if isMarker p then
                  go (acc + toString (builtins.head remaining)) (builtins.tail remaining) rest
                else
                  go (acc + p) remaining rest;
          in
          go "" resolved parts;
      };

  # ledgerLookup :: ledger -> "<id>.<attr>" -> { found, value }.
  # The ledger is { phase, outputs = { <id> = { <attr> = value; }; }; }. Only
  # single-level attrs are looked up (nested paths resolve via the executor's
  # ledger for the PoC scope).
  ledgerLookup =
    ledger: key:
    let
      parts = lib.splitString "." key;
      # rejoin all but the last segment as the resource id (ids contain dots).
      attr = lib.last parts;
      resource = lib.concatStringsSep "." (lib.init parts);
      outputs = ledger.outputs or { };
    in
    if (outputs ? ${resource}) && (outputs.${resource} ? ${attr}) then
      {
        found = true;
        value = outputs.${resource}.${attr};
      }
    else
      {
        found = false;
        value = null;
      };

  # resolve :: ledger -> value -> value.
  # Replace a __ref/__derived with its concrete value if all its inputs are in
  # the ledger; otherwise leave it as a placeholder. Recurses into plain
  # attrsets/lists.
  resolve =
    ledger: v:
    if isBuild v then
      v # a __build leaf is a known value (a store path); the executor realises it
    else if isRef v || isSensitiveRef v then
      let
        key = builtins.head (inputsOf v);
        hit = ledgerLookup ledger key;
      in
      if hit.found then hit.value else v
    else if isDerived v then
      let
        refs = v.__derived.__inputRefs or [ ];
        hits = map (r: ledgerLookup ledger (builtins.head (inputsOf r))) refs;
      in
      if builtins.all (h: h.found) hits && (v.__derived ? __render) then
        v.__derived.__render (map (h: h.value) hits)
      else
        v
    else if builtins.isAttrs v then
      builtins.mapAttrs (_: resolve ledger) v
    else if builtins.isList v then
      map (resolve ledger) v
    else
      v;
}
