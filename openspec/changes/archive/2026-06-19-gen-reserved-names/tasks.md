# Tasks: gen-reserved-names

## 1. Reserved-name guard in the emitter
- [x] 1.1 `internal/gen/emit.go`: define the reserved set `{ name, overrides,
      nivis }` and a helper that, for a given attribute/block name, returns the
      lambda formal to use (the real name, or a safe alias like `cfg_<name>` when
      reserved) plus the real config key.
- [x] 1.2 Lambda head: emit each input/block formal using its (possibly aliased)
      formal name, so `name`/`overrides`/`nivis` are never duplicated or shadowed.
- [x] 1.3 Required-presence throw: use the alias for the binding, keep the
      provider's real attribute name in the error message.
- [x] 1.4 config: emit the provider attribute under its REAL key, reading from the
      alias (required: `name = _cfg_name;`; optional: `// (if cfg_name == null
      then {} else { name = cfg_name; })`). The reserved instance `name` still
      threads via `inherit name config`.
- [x] 1.5 Generated file header: document any alias (real key -> formal) so the
      file stays a readable reference.

## 2. Unit test
- [x] 2.1 `internal/gen/emit_test.go`: a resource with a required `name` attr and
      an optional `overrides` attr. Assert: `name` and `overrides` each appear
      exactly once as a lambda formal; the emitted `config` sets `name = …` and
      `overrides = …` from the aliases; the output is deterministic.

## 3. Configure-free fake with a colliding attribute
- [x] 3.1 Add a fake resource whose schema has an attribute literally named `name`
      (and ideally one named `overrides`) plus a computed `id`, reusing
      `internal/fakeprovider`. Put it on a fake provider used by the e2e (extend an
      existing fake or add a `name`-attr resource).

## 4. Hermetic e2e
- [x] 4.1 `tests/e2e/codegen_test.go`: gen the colliding fake, `nix eval` the
      emitted constructor with the lib and a value for the `name` attr, assert:
      (a) it evaluates (no "duplicate formal", no eval error);
      (b) the produced resource has the right Nivis instance id AND the provider
      `name` value in its `config`.

## 5. Changelog
- [x] 5.1 `CHANGELOG.md` `[Unreleased]` `### Fixed`: the duplicate-`name`-formal
      fix (matches the proposal's `Changelog:` line).

## 6. Gate
- [x] 6.1 `gofmt`, `go build ./...`, `go test ./...` green.
- [x] 6.2 `bash tests/run-nix-tests.sh` + `bash tests/check-docs-ssot.sh` green.
- [x] 6.3 `openspec validate gen-reserved-names --strict`; archive; close
      beans-56tm with the cross-project handoff note (registry re-pins after the
      release; ~1379 constructors become valid).
