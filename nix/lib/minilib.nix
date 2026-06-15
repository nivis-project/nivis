# A minimal, self-contained subset of nixpkgs `lib`, implemented on builtins
# only. The PoC must evaluate without the Nix binary cache (CLAUDE.md §6), so we
# do not depend on <nixpkgs>. Swap this for real nixpkgs lib once the cache is
# reachable; the function signatures match.
let
  self = {
    concatStrings = builtins.concatStringsSep "";

    concatStringsSep = builtins.concatStringsSep;

    concatMapStringsSep = sep: f: xs: builtins.concatStringsSep sep (map f xs);

    concatLists = builtins.concatLists;

    concatMap = f: xs: builtins.concatLists (map f xs);

    mapAttrsToList = f: attrs: map (name: f name attrs.${name}) (builtins.attrNames attrs);

    optionalAttrs = cond: attrs: if cond then attrs else { };

    init = xs: builtins.genList (i: builtins.elemAt xs i) (builtins.length xs - 1);

    last = xs: builtins.elemAt xs (builtins.length xs - 1);

    # splitString: split a string on a literal separator into a list of strings.
    splitString =
      sep: s:
      let
        parts = builtins.split (self.escapeRegex sep) s;
      in
      # builtins.split interleaves matches (as lists) with literal strings; keep
      # only the literal string pieces.
      builtins.filter builtins.isString parts;

    # escapeRegex: escape regex metacharacters so a literal separator (e.g. ".")
    # is matched literally by builtins.split.
    escapeRegex =
      s:
      let
        meta = [
          "\\"
          "."
          "+"
          "*"
          "?"
          "["
          "]"
          "^"
          "$"
          "("
          ")"
          "{"
          "}"
          "|"
          "/"
        ];
        chars = builtins.filter builtins.isString (builtins.split "" s);
        escapeChar = c: if builtins.elem c meta then "\\" + c else c;
      in
      builtins.concatStringsSep "" (map escapeChar chars);
  };
in
self
