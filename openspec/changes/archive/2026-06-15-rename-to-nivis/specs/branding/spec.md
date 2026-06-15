# Spec delta: branding

## MODIFIED Requirements

### Requirement: Product name and tagline
README and docs SHALL present the product as **Nivis** with the tagline
**"Infrastructure as Nix Code"** and the payoff line **"All your base belongs to
Nix,"** noting its lineage (formerly `nixform`, then Terrae Nivis). A brand tokens
reference (`docs/BRAND.md`) SHALL record the colour and typography tokens.

#### Scenario: name, tagline, payoff, and tokens present
- WHEN the docs are inspected
- THEN README states the name **Nivis** + tagline + the payoff + the lineage note,
  and `docs/BRAND.md` lists the colour and type tokens.

### Requirement: Branded CLI splash
The `nivis` CLI SHALL print a branded splash (ASCII peak, "NIVIS", tagline) with
the ember accent for the prompt/"fixpoint reached" and ice-blue for resource
names, when run with no arguments. It SHALL emit plain (uncoloured) text when
`NO_COLOR` is set or output is not a TTY. Schema codegen is a subcommand of the
same binary (`nivis gen`), not a separate executable.

#### Scenario: splash on no-args invocation
- WHEN `nivis` is run with no arguments on a TTY
- THEN it prints the branded splash with ANSI colours and the wordmark "NIVIS".

#### Scenario: NO_COLOR / piped output is plain
- WHEN `nivis` runs with `NO_COLOR=1` or its output is piped
- THEN the splash contains no ANSI escape codes.

#### Scenario: codegen is a subcommand
- WHEN `nivis gen --provider <p> --out <dir>` is run
- THEN it generates the typed Nix constructors (the former `tn-gen`), from the one `nivis` binary.
