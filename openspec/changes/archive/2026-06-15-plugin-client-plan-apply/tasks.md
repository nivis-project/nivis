# Tasks: plugin-client-plan-apply

- [x] 1.1 Vendor `proto/tfplugin6.proto` (verbatim from terraform-plugin-go,
      go_package overridden) + `proto/generate.sh`; generate
      `internal/tfplugin6/*.pb.go`. `go build ./internal/tfplugin6` clean.
- [x] 1.2 `internal/plugin/manager.go`: handshake config (proto 6, tf6server
      magic cookie), a go-plugin `GRPCPlugin` whose `GRPCClient` returns
      `tfplugin6.NewProviderClient(conn)`, `Manager.Client(id, path)` that spawns
      + handshakes + pools by identity, and `Close()` that kills all.
- [x] 1.3 `internal/tfvalue/`: encode a resolved config map + provider schema
      into a `tfplugin6.DynamicValue` (msgpack), mapping unresolved `__ref`/
      `__derived` leaves to the protocol unknown; decode a DynamicValue result
      back into a Go attr map. Schema drives the tftypes.Object shape.
- [x] 1.4 `internal/plan/plan.go`: `SchemaFor` + `Plan(...)` ->
      PlanResourceChange; report unknown-after-apply attrs; render a
      human-readable plan; no side effects.
- [x] 1.5 `internal/apply/apply.go`: `Apply(...)` -> ApplyResourceChange; extract
      computed outputs; `state.Set` after success.
- [x] 1.6 Integration test `internal/plugin/integration_test.go`: builds the fake
      binaries, spawns via the manager, GetSchema -> Plan -> Apply for alpha and
      beta, asserts persisted outputs == deterministic derivations and
      unknown-at-plan; also asserts pooling by identity. Skips cleanly if the
      binaries can't be built.
- [x] 1.7 `go test ./...` and `go vet ./...` pass; integration test green.
      (IR conformance still 7/7.)
- [x] 1.8 `openspec validate plugin-client-plan-apply` passes; completes E3 —
      beans epic E3 and feature beans-uu26 marked done.
