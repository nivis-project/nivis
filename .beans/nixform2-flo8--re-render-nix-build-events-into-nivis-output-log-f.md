---
# nixform2-flo8
title: Re-render Nix build events into Nivis output (--log-format internal-json)
status: todo
type: feature
priority: normal
created_at: 2026-09-11T14:14:50Z
updated_at: 2026-09-11T14:30:38Z
blocked_by:
    - nixform2-7gdt
---

Follow-up to nixform2-7gdt / OpenSpec change `stream-run-progress`. Depends on
that change landing first: re-rendering needs a renderer that owns the terminal
before it has anywhere to draw.

## What

Consume `nix --log-format internal-json` from `nix eval` and
`nix-store --realise` and re-render it in Nivis's own visual language, instead
of the raw passthrough that `stream-run-progress` wires up at `verbose`.

Worth having: `resProgress` carries `[done, expected, running, failed]`, which
is a REAL denominator — the phase loop structurally cannot produce one, because
later phases reveal resources phase 0 cannot see. `resBuildLogLine` carries the
build's own stdout.

## The interface, as probed on nix 2.34.8

    @nix {"action":"start","id":..,"level":4,"parent":0,"type":109,
          "text":"querying info about '/nix/store/..' on 'https://cache.nixos.org'",
          "fields":["/nix/store/..","https://cache.nixos.org"]}
    @nix {"action":"result","id":..,"type":105,"fields":[3,3,0,0]}
    @nix {"action":"msg","level":3,"msg":"this derivation will be built:"}
    @nix {"action":"stop","id":..}

Three properties that make this fragile, and that the contract work exists to
manage:

1. `type` is a bare integer whose ENUM DEPENDS ON `action`: on `start` it is an
   ActivityType, on `result` it is a ResultType. Nothing in the payload says so.
2. `fields` is positional and unnamed. `[3,3,0,0]` is [done, expected, running,
   failed] only if you already know `type:105` is progress. The mapping lives in
   nix's C++ `logging.hh`, not in any documented contract. It is stable in
   practice because nix-output-monitor depends on it.
3. `resProgress` fires constantly — 14 identical `[0,0,0,0]` lines for a TRIVIAL
   derivation. Redraw throttling is not optional.

## Scope (all three parts are part of this work, not follow-ups)

1. **Degrade, never fail.** An unknown `type` is ignored, a malformed line is
   skipped, and a stream that stops parsing sensibly falls back to raw
   passthrough MID-BUILD. Nivis's decoration must never be able to break the
   user's actual build. Hard SHALL in the spec.

2. **A pinned, tested contract.** A conformance test that runs a real derivation
   under each supported nix version, captures the raw stream as a GOLDEN
   FIXTURE, and asserts the decoder extracts the expected facts. Fixtures keep
   the unit tests hermetic (CLAUDE.md section 6: no network); only the
   conformance run needs a live nix.

3. **A version map.** This is the `compat-tiers` pattern pointed at nix instead
   of providers — reuse its vocabulary:

   | nix range    | tier                   | behaviour                      |
   |--------------|------------------------|--------------------------------|
   | below floor  | unsupported            | raw passthrough, no re-render  |
   | pinned range | verified               | fixtures in repo, CI-checked   |
   | newer        | compatible by design   | tolerant decode, degrade freely|

   Nivis reads `nix --version` at startup so it knows its tier and can report it
   at `verbose`.

## Inherited invariant

Raw passthrough and the live region are MUTUALLY EXCLUSIVE — nix's own progress
bar and Nivis's in-place region fight for the same cursor. `stream-run-progress`
establishes the rule; this change keeps it:

    quiet    nix suppressed            live off
    info     nix re-rendered           live ON   (Nivis owns the terminal)
    verbose  raw passthrough           live off  (nix owns the terminal)
    debug    raw + internal-json       live off

## Spike before implementing

Confirm `nix-store --realise` (the old CLI) honours `--log-format internal-json`
the same way `nix build` does, and whether `--print-build-logs` is additionally
needed to get `resBuildLogLine`.

CI automation for keeping the contract current is tracked separately.


---

**Not closed by the `stream-run-progress` OpenSpec change.** That change is a
prerequisite, not this bean's implementation. `beans list -S
"stream-run-progress"` matches this bean because the change is named above;
that match is a dependency reference, not an implementation link.
