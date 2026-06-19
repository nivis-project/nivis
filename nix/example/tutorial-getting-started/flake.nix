{
  description = "Nivis getting-started tutorial: the headline two-provider round trip";

  # Pinned to the nivis release that scaffolded this tutorial, so the library and
  # your `nivis` binary agree. nivistutor rewrites @NIVIS_REF@ to its own build's
  # tag when it writes this file; if you see the literal token, replace it with a
  # nivis ref, e.g. github:wearetechnative/nivis/v0.4.3.
  inputs.nivis.url = "@NIVIS_REF@";

  outputs =
    { self, nivis }:
    let
      lib = nivis.lib;
    in
    {
      # `nivis plan` evaluates this attr by default (--attr nivis.plan). It is a
      # function of the injected outputs ledger, exactly like the upstream repo.
      nivis.plan = import ./config.nix { nivis = lib; };
    };
}
