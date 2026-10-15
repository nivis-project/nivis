# A TEST FIXTURE, not a recommendation. It exists so the VM tier can drive nivis
# against a real, unmodified third-party provider rather than against the in-repo
# fakes, which encode our own reading of the protocol (see bean nixform2-q29p).
#
# It is not a use case the project suggests: nivis is for infrastructure, and a
# postgres role is here because postgres is small, fast, deterministic and in
# nixpkgs. `cycle.nix` and `build-probe.nix` are fixtures in this directory on the
# same footing.
#
# The provider `source` is a FILESYSTEM PATH, not a registry address, because the
# nixosTest that drives this has no network. internal/registry/resolver.go uses an
# existing path verbatim with no registry interaction, so this exercises the
# executor and the provider, and deliberately NOT the registry.
#
#   nivis apply --attr nivis.postgres \
#     --var provider_path=/nix/store/…/terraform-provider-postgresql_1.26.0
#
# The provider path and the connection settings are variables so the test can
# supply the store path it built and the socket the VM listens on, without this
# file knowing either.
{ nivis }:
ledger:
let
  inherit (nivis)
    mkResource
    mkProvider
    mkVars
    toIR
    ;

  vars = mkVars {
    # The provider binary, as a store path. No default: a wrong or absent value
    # must fail loudly rather than silently reach for a registry.
    provider_path = {
      type = "str";
    };
    pg_host = {
      type = "str";
      default = "localhost";
    };
    pg_user = {
      type = "str";
      default = "postgres";
    };
    pg_database = {
      type = "str";
      default = "postgres";
    };
  } (ledger.vars or { });

  # One role. The assertion the test makes is not about this resource's values;
  # it is that a plan taken straight after applying reports no changes, which is
  # the strongest cheap statement that nivis and a real provider agree about what
  # was applied. See the change's design, D4.
  role = mkResource {
    provider = "postgresql";
    type = "postgresql_role";
    name = "appuser";
    config = {
      name = "appuser";
      login = true;
    };
  };
in
toIR {
  providers = {
    postgresql = mkProvider {
      source = vars.provider_path;
      config = {
        host = vars.pg_host;
        username = vars.pg_user;
        database = vars.pg_database;
        sslmode = "disable";
        # The connected user IS a superuser here, and saying so lets the
        # provider use the statements it would normally avoid.
        superuser = true;
      };
    };
  };
  resources = [ role ];
  inherit ledger;
}
