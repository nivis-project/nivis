# Tasks: rich-fake-provider

- [x] 1.1 `internal/fakeproviderx/fakeproviderx.go`: a tfprotov6 fake-provider
      helper supporting rich-typed attributes (list/map/object); Apply takes/
      returns plain Go values and uses internal/tfcodec to bridge to tftypes
      (exercising the codec from the provider side too). Plan -> computed unknown;
      apply -> computed known. Separate from the string-only fakeprovider base.
- [x] 1.2 `cmd/provider-delta/main.go`: resource `delta_thing` — inputs `tags`
      map(string), `ports` list(number), `label` string(opt); computed `id`
      string, `endpoints` list(string) (ep-<port>-<counter> per port), `meta`
      object({region,count}) from tags. Deterministic (TERRAE_NIVIS_FAKE_COUNTER);
      tf6server.
- [x] 1.3 `internal/plugin/delta_test.go`: spawns the REAL provider-delta via the
      manager; plan asserts id/endpoints/meta unknown; apply with
      tags={env=prod}, ports=[80,443] asserts endpoints=["ep-80-0","ep-443-0"],
      meta={region:prod,count:1}, id=delta-0 — full encode->provider->decode
      round trip for collections + nested object.
- [x] 1.4 `go test ./...` + `go vet ./...` pass; nix tests + IR conformance green;
      existing fakes unchanged; gofmt clean.
- [x] 1.5 `openspec validate rich-fake-provider` passes; E4a (nixform2-kxlp)
      progress noted; de-risks the AWS plan.
