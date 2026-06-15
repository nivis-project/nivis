# mkProvider builds a provider declaration in the IR shape `{ source, config }`.
# It is the ergonomic, validated front door for the `providers` argument to
# `toIR`: `providers.aws = mkProvider { source = "..."; config = { region = ...; }; }`.
#
# Provider config is a raw attribute tree (the provider validates it at
# Configure time against its own schema). Nested provider blocks — e.g.
# `default_tags`, `assume_role`, `endpoints` — are ordinary nested attrsets/lists
# inside `config`; `toIR` serializes them and the executor's object-type
# construction maps them onto the provider's schema. Config may contain
# `__ref`/`__derived` leaves; `toIR` resolves them against the outputs ledger each
# phase, exactly as it does for resource config.
{ lib }:
# mkProvider :: { source, config ? {} } -> { source, config }
# `source` defaults to null (rather than being a required arg) so that omitting
# it yields our own named error rather than Nix's generic missing-argument one.
{
  source ? null,
  config ? { },
}:
if source == null || source == "" then
  throw "mkProvider: `source` is required and must be a non-empty string (the provider address, e.g. \"registry.opentofu.org/hashicorp/aws\")"
else if !(builtins.isString source) then
  throw "mkProvider: `source` must be a string, got ${builtins.typeOf source}"
else
  {
    inherit source config;
  }
