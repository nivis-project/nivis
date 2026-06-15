# A deliberately CYCLIC variant of the headline topology, for the e2e's
# cycle-rejection assertion (docs/TESTING.md). A.label depends on C.value and
# C.label depends on A.value, so neither can ever become ready: the driver must
# reach fixpoint with A and C unapplied and report them.
{ terraeNivis }:
ledger:
let
  inherit (terraeNivis) mkResource toIR str;

  A = mkResource {
    provider = "alpha";
    type = "alpha_token";
    name = "A";
    config = {
      # cyclic: A's input derives from C's output
      label = str [
        "from-c-"
        (C.refAttr "value")
      ];
    };
  };

  C = mkResource {
    provider = "alpha";
    type = "alpha_token";
    name = "C";
    config = {
      # cyclic: C's input derives from A's output
      label = str [
        "from-a-"
        (A.refAttr "value")
      ];
    };
  };
in
toIR {
  providers = {
    alpha = {
      source = "./bin/provider-alpha";
      config = { };
    };
  };
  resources = [
    A
    C
  ];
  inherit ledger;
}
