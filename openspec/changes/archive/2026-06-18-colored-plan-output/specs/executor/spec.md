# Spec delta: executor

## ADDED Requirements

### Requirement: Apply result groups applied nodes by phase
The phase driver's apply result SHALL report, in addition to the flat ordered list
of applied ids and the phase count, the ids applied in **each** phase, in phase
order, and for each applied id whether it was a **resource** (planned/applied) or
a **datasource** (read). This is reporting metadata only: it SHALL NOT change which
nodes are applied, the order, or any apply behaviour. It exists so a caller can
render the fixpoint (which nodes resolved in which phase) and distinguish a read
from a create.

#### Scenario: per-phase grouping reflects the fixpoint
- GIVEN a run that applies resources across three phases (a Nix-mediated chain)
- WHEN it completes
- THEN the result reports three phase groups, each listing the ids applied in that phase, and their concatenation in phase order equals the flat applied list.

#### Scenario: a datasource read is marked as a read
- GIVEN a run that reads a datasource and applies a resource
- WHEN it completes
- THEN the result marks the datasource id as a read and the resource id as an applied resource.
