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
    # The collation the managed database is created with. postgresql treats it
    # as identity: changing it is a replacement, not an alteration. That is what
    # lets the test drive a cascade from the CLI without editing this file.
    #
    # Use "C" and "C.utf8" as the two values. NOT "POSIX": glibc treats POSIX as
    # an alias for C, so postgres stores C whatever you asked for, and nivis
    # would then propose the same replacement on every run forever. Measured,
    # not assumed (bean nixform2-nwnf).
    db_collate = {
      type = "str";
      default = "C";
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
  # A two-link chain whose reference lands in an attribute postgresql treats as
  # identity. Replacing the database makes `schema.database` unknown, and a
  # provider turns an unknown in an identity-bearing attribute into a
  # replacement, so the schema must be replaced with it.
  #
  # Measured against this provider before it was relied on (bean nixform2-nwnf):
  # `schema.database` unknown replaces, `schema.owner` unknown does not. The
  # second is why nivis asks the provider rather than assuming every holder of a
  # replaced object must go.
  database = mkResource {
    provider = "postgresql";
    type = "postgresql_database";
    name = "app";
    config = {
      name = "appdb";
      lc_collate = vars.db_collate;
      lc_ctype = vars.db_collate;
      # A database can only be created with a non-default collation from a
      # template that has none of its own.
      template = "template0";
    };
  };

  schema = mkResource {
    provider = "postgresql";
    type = "postgresql_schema";
    name = "app";
    config = {
      name = "appschema";
      database = database.refAttr "name";
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
  resources = [
    role
    database
    schema
  ];
  inherit ledger;
}
