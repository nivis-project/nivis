# Nivis tutorial: what's new in 0.4

This is a self-contained Nivis project. It tours the daily-driver features added
in the 0.4 line, against the in-repo fake providers (no cloud, no credentials):
typed variables, a datasource, the round trip across phases, and stack outputs.
Read `config.nix`, then run the commands below and watch what each one does.

## Before you start

You need `nivis` and the fake providers on your PATH. The one-liner that gives you
both (and `nivistutor`) in a throwaway shell:

```sh
nix shell github:nivis-project/nivis#nivis github:nivis-project/nivis#tutor
```

The configs reference the providers by bare name (`source = "provider-alpha"`),
so `nivis` finds them on PATH.

## Run it

From this directory:

```sh
nivis plan   --var env=prod
nivis apply  --var env=prod
nivis output --var env=prod
```

What to notice:

- **Variables.** `env` is required (try omitting `--var env=...` and read the
  error). `replicas` has a default of 2. Override it with `--var replicas=5`.
- **Datasource.** `alpha_lookup` reads "existing" infrastructure and its result
  flows into the token's label.
- **Round trip.** The beta record's `from` is built in Nix from the alpha token's
  apply-time value, so apply resolves across more than one phase. Watch the phase
  headings.
- **Outputs.** `nivis output` prints the named values; `nivis output endpoint`
  prints a single one; add `--json` for a machine-readable form.

## Next

Edit `config.nix`, change a value, and re-run `nivis plan` to see the diff. When
you are done, `nivis destroy` tears it down.
