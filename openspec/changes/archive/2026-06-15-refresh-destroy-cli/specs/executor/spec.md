# Spec delta: executor

## ADDED Requirements

### Requirement: Destroy in reverse dependency order
The executor SHALL destroy resources in reverse dependency order (a dependent
before the resources it depends on), calling the provider to delete each and
removing it from the state store. It SHALL refuse to destroy a resource marked
`lifecycle.preventDestroy` with an actionable error naming it.

#### Scenario: reverse-order teardown
- GIVEN applied resources A, then B (depends on A), then C (depends on B)
- WHEN destroy runs
- THEN the provider is asked to delete C, then B, then A, in that order
- AND each is removed from the state store.

#### Scenario: preventDestroy is honored
- GIVEN a resource with `lifecycle.preventDestroy = true`
- WHEN destroy targets it
- THEN it fails with an error naming the resource and the resource remains in state.

### Requirement: Refresh reconciles via ReadResource without planning
The executor SHALL refresh by calling `ReadResource` for each resource in state
with its stored state and writing back the reconciled result. Refresh SHALL NOT
plan or apply changes.

#### Scenario: refresh leaves a converged state unchanged
- GIVEN state for resources whose providers return the same values on read
- WHEN refresh runs
- THEN each resource's state is unchanged and no apply is performed.

### Requirement: Command-line interface
The system SHALL provide a `nixform` CLI with `plan`, `apply`, `destroy`,
`refresh`, and `state` (`list`, `show`, `rm`) subcommands, a `--target` id
filter, and `--state`/`--flake` options. `plan`/`apply` drive the phased-eval
loop; `destroy`/`refresh` use their engines.

#### Scenario: state list shows applied resources
- GIVEN a state store with applied resources
- WHEN `nixform state list` runs
- THEN it prints each resource id.

#### Scenario: state rm removes one resource
- GIVEN a state store containing resource R
- WHEN `nixform state rm R` runs
- THEN R is no longer in the store.
