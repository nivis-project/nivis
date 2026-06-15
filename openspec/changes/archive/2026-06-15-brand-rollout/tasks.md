# Tasks: brand-rollout

- [x] 1.1 Vendored production SVGs into `assets/` (terrae-nivis-emblem.svg,
      terrae-nivis-glyph.svg), byte-for-byte from the handoff.
- [x] 1.2 Icons from the glyph: `assets/favicon.svg`; `assets/favicon.ico` (32px,
      rsvg-convert + magick); `assets/apple-touch-icon.png` (180px glyph on a
      `#0E3157` tile, ~14% padding) with source `docs/assets/apple-touch-icon.svg`.
      Commands recorded in `docs/BRAND.md`.
- [x] 1.3 README hero banner: authored `docs/assets/banner.svg` to the handoff
      `09` token spec; rasterised to `docs/assets/banner.png` (1280×640) using
      Cinzel + IBM Plex Mono (fetched via nixpkgs for the render). Shown at the
      top of `README.md`. Visually verified on-brand.
- [x] 1.4 `docs/BRAND.md`: colour tokens, typography, CLI colours, logo rules,
      and the icon/banner regeneration commands.
- [x] 1.5 CLI splash in `cmd/tn` (`splash.go`): ASCII peak + "TERRAE NIVIS" +
      tagline + version, ember `❯`, ice-blue, dim secondary; shown on no-args and
      `tn --version`. Honours NO_COLOR + non-TTY (golang.org/x/term) -> plain.
      `splash_test.go` asserts the text + no-ANSI-when-non-TTY + NO_COLOR.
- [x] 1.6 README: product name **Terrae Nivis** + tagline + "formerly nixform"
      note, banner on top.
- [x] 1.7 `go test ./...` + `go vet ./...` pass; nix tests + IR conformance
      green; gofmt clean.
- [x] 1.8 `openspec validate brand-rollout` passes; beans-b2by linked.
- [x] DEFERRED -> beans: docs-site og:image + theme (nixform2-9qgf, no site
      exists); binary rename tn -> nivis (nixform2-ijon — DECIDED: keep tn).
