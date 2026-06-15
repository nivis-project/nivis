# Tasks: codec-collections

- [x] 1.1 Extended the codec to handle list/set/tuple (from []interface{}) and
      map/object (from map[string]interface{}), recursing into element/attribute
      types; symmetric decode. Also fixed a latent bug: tftypes Number must
      decode via *big.Float, not *float64 (the scalar fakes are string-only so it
      was never hit).
- [x] 1.2 Factored the shared recursion into `internal/tfcodec` (GoToValue/
      ValueToGo); both `internal/tfvalue` (v6) and `internal/provider/v5` now
      delegate to it — no duplication, v5/v6 stay in lockstep by construction.
- [x] 1.3 `internal/tfcodec/tfcodec_test.go`: round-trips for string/number/bool,
      list, set, map, tuple, nested object (incl. a list inside), object-fills-
      missing-attrs-with-null, unsupported-type error, null + type-mismatch errors.
- [x] 1.4 `internal/provider/v5/tfvalue5_test.go`: v5 encodeState/decodeState
      round-trip over an object with map + list attributes (exercises the v5
      DynamicValue wrappers, not just the shared codec).
- [x] 1.5 `go test ./...` + `go vet ./...` pass; nix tests + IR conformance green;
      scalar fakes unchanged; gofmt clean.
- [x] 1.6 `openspec validate codec-collections` passes; beans-guxs closed.
