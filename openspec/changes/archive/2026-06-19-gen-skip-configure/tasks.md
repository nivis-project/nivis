# Tasks: gen-skip-configure

## 1. Manager: a configure-free schema client
- [x] 1.1 `internal/plugin/manager.go`: factor the spawn/handshake/dispense/
      version-negotiate/pool body of `Client` into a private helper that returns
      the `provider.Client` WITHOUT configuring.
- [x] 1.2 `Client(identity, path, config)` = helper + `cl.Configure(...)` (exactly
      as today; unchanged behaviour for plan/apply/refresh/destroy).
- [x] 1.3 New `ClientForSchema(identity, path)` = helper only (no Configure).
      Pooling by identity preserved.

## 2. gen uses the schema path
- [x] 2.1 `cmd/nivis/gen.go`: replace `mgr.Client(identity, providerPath, map{})`
      with `mgr.ClientForSchema(identity, providerPath)`. `gen.Fetch` unchanged.

## 3. Configure-rejecting fake provider
- [x] 3.1 Add `cmd/provider-epsilon` (a fake whose `ConfigureProvider` returns an
      ERROR diagnostic on a null/empty config but serves `GetProviderSchema`
      normally), reusing the `internal/fakeprovider` machinery where possible.
      Give it one resource type with a couple of attrs so `gen.Fetch` has output.
- [x] 3.2 Add `cmd/provider-epsilon` to the `#fake-providers` flake package's
      subPackages so it lands on the `nix shell .#fake-providers` PATH.

## 4. E2E proof
- [x] 4.1 `tests/e2e/codegen_test.go`: a new test that builds provider-epsilon on
      PATH and asserts:
      - `mgr.Client("epsilon", "provider-epsilon", map{})` FAILS at configure
        (guards that plan/apply still enforces Configure);
      - `mgr.ClientForSchema("epsilon", "provider-epsilon")` SUCCEEDS and
        `gen.Fetch` returns the resource(s) with the expected type/attrs.
- [x] 4.2 Keep `TestCodegenAgainstFake` (the all-null-configure-OK provider-alpha
      case) passing, now via `ClientForSchema` too.

## 5. Changelog + release
- [x] 5.1 `CHANGELOG.md` `[Unreleased]`: a `### Fixed` entry (gen no longer
      configures before schema; unblocks credential-requiring providers).
- [ ] 5.2 (At release time, separately) cut **0.4.2** so the registry can re-pin.

## 6. Gate
- [x] 6.1 `gofmt`, `go build ./...`, `go test ./...` green.
- [x] 6.2 `bash tests/run-nix-tests.sh` + `bash tests/check-docs-ssot.sh` green
      (changelog gate: this change's proposal has a `Changelog:` line).
- [x] 6.3 `openspec validate gen-skip-configure --strict`; archive; close
      beans-jcpm with the cross-project handoff note.
