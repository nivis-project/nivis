#!/usr/bin/env python3
"""IR conformance checker — the executable form of the docs/IR-CONTRACT.md
"Validation" obligations (OpenSpec change define-ir-contract, task 1.8).

Two layers, both of which the future Nix `toIR` property test and Go `IngestIR`
validator must also enforce:

  1. STRUCTURAL — validate against docs/ir-schema.json (JSON Schema 2020-12).
     Covers leaf shapes (__ref/__derived/__sensitiveRef), required fields,
     and the "no count/for_each in meta" rule (meta.additionalProperties=false).
  2. REFERENTIAL — rules JSON Schema cannot express:
       - resource ids are unique,
       - every resource.provider is a declared provider id,
       - every edge endpoint (from/to) is an existing resource id,
       - every __ref/__sensitiveRef target resource exists.
     Each failure names the offending resource/edge/path (contract requirement:
     "malformed IR is rejected with identity").

Usage:
  check.py validate <ir.json>        # validate one IR; exit 0 ok, 1 on error
  check.py test                      # run the fixture suite (valid/ + invalid/)
"""

import json
import sys
from pathlib import Path

try:
    from jsonschema import Draft202012Validator
except ImportError:
    sys.stderr.write("error: python 'jsonschema' package required\n")
    sys.exit(2)

HERE = Path(__file__).resolve().parent
REPO = HERE.parent.parent
SCHEMA_PATH = REPO / "docs" / "ir-schema.json"
FIXTURES = HERE / "fixtures"


def load_schema():
    return json.loads(SCHEMA_PATH.read_text())


def strip_annotations(node):
    """Drop underscore-prefixed test-annotation keys (_comment, _expect_*) so
    fixtures can self-document without violating additionalProperties. Real IR
    never uses leading-underscore keys except the reserved __ref/__derived/
    __sensitiveRef (double underscore), which are preserved."""
    if isinstance(node, dict):
        return {
            k: strip_annotations(v)
            for k, v in node.items()
            if not (k.startswith("_") and not k.startswith("__"))
        }
    if isinstance(node, list):
        return [strip_annotations(v) for v in node]
    return node


def _walk_leaves(node, path):
    """Yield (path, ref_obj) for every __ref/__sensitiveRef leaf in a config tree."""
    if isinstance(node, dict):
        if "__ref" in node:
            yield (path, "__ref", node["__ref"])
            return
        if "__sensitiveRef" in node:
            yield (path, "__sensitiveRef", node["__sensitiveRef"])
            return
        if "__derived" in node:
            return  # derived inputs are id.attr strings, validated structurally only
        for k, v in node.items():
            yield from _walk_leaves(v, path + [k])
    elif isinstance(node, list):
        for i, v in enumerate(node):
            yield from _walk_leaves(v, path + [i])


def _deepest(e):
    """Follow anyOf/oneOf/if-then context to the most specific sub-error -- the
    one with the longest absolute path (deepest into the document). This makes a
    malformed marker leaf (e.g. a __ref missing 'path') report at config/label
    naming 'path', instead of a generic 'not valid under any of the given
    schemas' on the enclosing object."""
    best = e
    while best.context:
        # Among context errors, prefer the one reaching furthest into the doc;
        # break ties toward a non-generic message.
        cand = max(
            best.context,
            key=lambda c: (
                len(list(c.absolute_path)),
                "is not valid under any of" not in c.message,
            ),
        )
        if list(cand.absolute_path) == list(best.absolute_path) and not cand.context:
            best = cand
            break
        best = cand
    return best


def structural_errors(ir, schema):
    v = Draft202012Validator(schema)
    out = []
    for e in sorted(v.iter_errors(ir), key=lambda e: list(e.absolute_path)):
        leaf = _deepest(e)
        loc = "/".join(str(p) for p in leaf.absolute_path) or "<root>"
        out.append(f"structural: at {loc}: {leaf.message}")
    return out


def referential_errors(ir):
    errs = []
    resources = ir.get("resources", [])
    providers = ir.get("providers", {})

    # unique ids
    seen = {}
    ids = set()
    for r in resources:
        rid = r.get("id")
        if rid in seen:
            errs.append(f"referential: duplicate resource id '{rid}'")
        seen[rid] = True
        ids.add(rid)

    # provider declared
    for r in resources:
        prov = r.get("provider")
        if prov not in providers:
            errs.append(
                f"referential: resource '{r.get('id')}' uses undeclared provider '{prov}'"
            )

    # edge endpoints exist
    for e in ir.get("edges", []):
        for end in ("from", "to"):
            if e.get(end) not in ids:
                errs.append(
                    f"referential: edge {end}->'{e.get(end)}' "
                    f"(from={e.get('from')} to={e.get('to')} via={e.get('via')}) "
                    f"names a non-existent resource id"
                )

    # __ref / __sensitiveRef targets exist
    def check_refs(tree, owner):
        for path, kind, ref in _walk_leaves(tree, []):
            tgt = ref.get("resource")
            if tgt not in ids:
                p = ".".join(str(x) for x in path)
                errs.append(
                    f"referential: {kind} in {owner} at path [{p}] "
                    f"targets non-existent resource '{tgt}'"
                )

    for r in resources:
        check_refs(r.get("config", {}), f"resource '{r.get('id')}'")
    for c in ir.get("nixConsumers", []):
        check_refs(c.get("value", {}), f"nixConsumer '{c.get('id')}'")

    return errs


def validate(ir, schema):
    """Return list of error strings (empty == valid)."""
    errs = structural_errors(ir, schema)
    # Referential checks only make sense if the top-level shape parsed enough.
    if isinstance(ir, dict):
        errs += referential_errors(ir)
    return errs


def cmd_validate(argv):
    if len(argv) != 1:
        sys.stderr.write("usage: check.py validate <ir.json>\n")
        return 2
    ir = strip_annotations(json.loads(Path(argv[0]).read_text()))
    errs = validate(ir, load_schema())
    if errs:
        for e in errs:
            print(e)
        return 1
    print("ok: IR conforms to docs/ir-schema.json and referential rules")
    return 0


def cmd_test(_argv):
    schema = load_schema()
    failures = 0
    total = 0

    for f in sorted((FIXTURES / "valid").glob("*.json")):
        total += 1
        ir = strip_annotations(json.loads(f.read_text()))
        errs = validate(ir, schema)
        if errs:
            failures += 1
            print(f"FAIL valid/{f.name}: expected conforming, got:")
            for e in errs:
                print(f"    {e}")
        else:
            print(f"ok   valid/{f.name}")

    for f in sorted((FIXTURES / "invalid").glob("*.json")):
        total += 1
        raw = json.loads(f.read_text())
        want = raw.get("_expect_error_contains")
        ir = strip_annotations(raw)
        errs = validate(ir, schema)
        joined = " | ".join(errs)
        if not errs:
            failures += 1
            print(f"FAIL invalid/{f.name}: expected rejection, but it validated")
        elif want and want not in joined:
            failures += 1
            print(
                f"FAIL invalid/{f.name}: rejected, but no error mentioned "
                f"'{want}'. errors: {joined}"
            )
        else:
            print(f"ok   invalid/{f.name} (rejected; mentions '{want}')")

    print(f"\n{total - failures}/{total} fixtures passed")
    return 1 if failures else 0


def main():
    if len(sys.argv) < 2:
        sys.stderr.write(__doc__)
        return 2
    cmd, rest = sys.argv[1], sys.argv[2:]
    if cmd == "validate":
        return cmd_validate(rest)
    if cmd == "test":
        return cmd_test(rest)
    sys.stderr.write(f"unknown command '{cmd}'\n")
    return 2


if __name__ == "__main__":
    sys.exit(main())
