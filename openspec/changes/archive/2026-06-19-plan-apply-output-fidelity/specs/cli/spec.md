# Spec delta: cli

## MODIFIED Requirements

### Requirement: Plan and apply output is colorised by change type and grouped by phase
The CLI SHALL render plan, apply, and destroy output with the change type shown by
a marker and, on a color-capable terminal, a color: `+` create (green), `~` update
(yellow), `-/+` replace (red+green), `-` destroy (red), `=` no-op (dim), and a
datasource read with a distinct `r` marker in a read (dim) color so a read is
visibly not a create. Apply output SHALL group applied nodes under the phase they
resolved in, so the phased fixpoint is visible, and SHALL retain a count summary.
Apply output SHALL render each applied node by the **operation it actually
resolved as** (the op the phase driver reports for that node), so an in-place
update shows `~`, a replacement shows `-/+`, a no-op shows `=`, and only a real
create shows `+`; it SHALL NOT render every applied node as a create.

Color SHALL be gated exactly as the existing `colorEnabled` helper specifies: when
the output writer is not a TTY (e.g. piped or redirected) or when `NO_COLOR` is
set, the output SHALL contain **no ANSI escape codes**, while the markers, the
text, the counts, and the phase grouping SHALL be identical to the colored form
(so scripts and tests see stable output). The output SHALL be written through the
command's writer (capturable), not the process stdout directly.

#### Scenario: change types are colored on a TTY
- GIVEN a plan with a create, an update, a replace, and a no-op, rendered to a color-capable writer
- WHEN it is printed
- THEN each line carries its marker and an ANSI color appropriate to its change type.

#### Scenario: no color when piped or NO_COLOR is set
- GIVEN the same plan rendered to a non-TTY writer, or with `NO_COLOR` set
- WHEN it is printed
- THEN the output contains no ANSI escape codes, but the same markers, text, counts, and phase grouping as the colored form.

#### Scenario: apply output is grouped by phase
- GIVEN an apply that resolved resources across multiple phases
- WHEN it prints
- THEN resources are listed under their phase heading in phase order, with a summary of the total applied and the phase count.

#### Scenario: a datasource read is shown distinctly
- GIVEN an apply that read a datasource and created a resource
- WHEN it prints
- THEN the datasource line uses the `r` read marker (and read color on a TTY), distinct from the `+` create marker.

#### Scenario: apply shows the real op, not always create
- GIVEN a second apply of an already-applied, unchanged stack
- WHEN it prints
- THEN each resource is shown with its true op (a no-op `=` or update `~`), not `+` create.
