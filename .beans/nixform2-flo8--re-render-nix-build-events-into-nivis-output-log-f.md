---
# nixform2-flo8
title: Re-render Nix build events into Nivis output (--log-format internal-json)
status: completed
type: feature
priority: normal
created_at: 2026-09-11T14:14:50Z
updated_at: 2026-09-14T21:35:23Z
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


---

## Spike results, 2026-09-14 (nix 2.34.8)

Both questions this bean posed are answered, empirically. One answer is the
opposite of what the bean assumed.

### 1. Does `nix-store --realise` honour `--log-format internal-json`?

**Yes**, identically to `nix build`. It emits the same `@nix {...}` stream.

### 2. Is `--print-build-logs` additionally needed for the build's own log?

**No — and passing it BREAKS the command.** `nix-store` rejects it:

    error: unknown flag '--print-build-logs'
    Try 'nix-store --help' for more information.

It is a new-CLI (`nix build`) flag and does not exist on `nix-store`. The build
log arrives regardless, as `resBuildLogLine` events. A probe whose builder
printed two stdout lines and one stderr line produced exactly three:

    result type 101  fields: ["probe: line one to stdout"]
    result type 101  fields: ["probe: line two to stdout"]
    result type 101  fields: ["probe: a line to stderr"]

So stdout AND stderr both come through, already split into lines.

### The event semantics we depend on, all observed

    start  type 105  actBuild         fields[0] = the .drv being built
                                      e.g. "/nix/store/jcz1...-probe.drv"
    result type 101  resBuildLogLine  fields[0] = one line of build output
    result type 105  resProgress      fields = [done, expected, running, failed]
                                      observed: [0,1,0,0] -> [0,1,1,0] -> [1,2,0,0]
    result type 106  resSetExpected   fields = [activityType, expected]
    start  type 0                     generic activity, `text` carries the label
    start  type 101  actFileTransfer  fields[0] = the URL
    start  type 102  actRealise
    start  type 103  actCopyPaths
    start  type 104  actBuilds
    start  type 109  actQueryPathInfo fields = [storePath, substituter]
    stop             (no type)        closes the activity with that id

`resProgress` gives the REAL denominator the phase loop structurally cannot:
[done, expected] over derivations. That is the number worth putting on screen.

### Confirms the fragility this bean was filed about

`type` is meaningless without `action`: 105 on a `start` is actBuild, 105 on a
`result` is resProgress. 101 on a `start` is actFileTransfer, 101 on a `result`
is resBuildLogLine. Nothing in the payload says so. A decoder that switches on
`type` alone is wrong in a way that looks like it works.

### Volume

53 lines for a probe that builds one trivial derivation and prints three lines.
Of those, 22 are `resProgress`, most of them repeats of the same tuple. The
throttled redraw `stream-run-progress` already established is what makes this
usable; without it, a NixOS image would be a redraw storm.

OpenSpec change: `render-nix-build-events`.


---

## Scope narrowed, 2026-09-14

Part 3 of this bean — the `compat-tiers`-style nix version map — **moves to
nixform2-lpqk**. This bean keeps the decoder, the rendering, degrade-never-fail,
and ONE golden fixture generated from the pinned nix.

Reason the original bundling argument does not hold: this bean argued that a
re-renderer shipped without a version story "breaks silently on a nix upgrade".
It does not, because degrade-never-fail is part of THIS change. An unknown
`type` is ignored and a stream that stops parsing falls back to raw passthrough,
which is visible degradation, not silent breakage.

What the version map actually buys is EARLY DETECTION and honest tier
reporting — and detection needs something to exercise the map periodically.
That something is nixform2-lpqk's CI matrix. A map without the matrix that
watches it is a table, not a guard. The map and its matrix are one piece of
work, so they belong in one bean.

The single golden fixture stays here: it is how the decoder is tested
hermetically (CLAUDE.md section 6 — no network in the test path), not part of
the version story.

Also kept here: report the DETECTED nix version at `verbose`, so a user on an
untested nix can see that fact even before the map exists.

OpenSpec change: `render-nix-build-events`.


---

## Summary of Changes

OpenSpec change: `render-nix-build-events`. `run-progress` modified (1
requirement) and extended (3 new).

New `internal/nixlog` decodes Nix's `--log-format internal-json` stream. The
realise path asks for that stream at the default verbosity and reports what it
says, so the live region can stay up while a build runs. At `verbose` Nivis
still hands the terminal to Nix, unchanged.

A running build now shows the derivation, `[done/expected]`, elapsed time, and
the latest line of its own output — layout "B", two rows, truncated to the
terminal width.

### The bug the integration test caught

The fixture tests all passed while the real path reported nothing. The
streaming layer counted "the snapshot did not change" as "the line could not be
read", and gave up on the format after 20 such lines. A single narinfo query
emits about fourteen well-formed, identical progress tuples, so it degraded to
raw passthrough BEFORE the build began.

It would have shipped silently: falling back is not an error, so the only
symptom was that build progress never appeared. Replaying the captured output
through a fresh decoder gave the right answer every time, which is exactly why
a fixture test could not find it.

`Decoder.Line` now returns `changed` AND `understood` separately, with the
distinction documented at the call site, and three regression tests pin it —
including one that feeds fifty identical progress events and asserts none is
reported as unreadable.

### Two smaller traps, both rediscovered the hard way

- The integration probe's derivation name must carry a RANDOM token.
  `t.TempDir()` ends in a repeating counter, so a dir-derived name is already
  built on the second run and the test silently observes no build.
  `nix/example/build-probe.nix` guards against this for the same reason.
- Two renderer tests raced: they waited for `rows > 0`, already true from an
  earlier committed line, so they closed before the tick that paints the
  detail. They wait for the content now.

### What the spike settled, now encoded as requirements

- `nix-store --realise` honours `--log-format internal-json`; `--print-build-logs`
  does NOT exist there and passing it fails the command. The build log arrives
  as `resBuildLogLine` regardless.
- An event's meaning needs `action` AND `type`: 105 is a build under `start`
  and progress under `result`; 101 is a download under `start` and a log line
  under `result`.
- Progress must be scoped to the activity that aggregates builds. A substituter
  query reporting "3 of 3" before any build starts is real, and reading it as
  build progress announces a build finished before it began.
- The expected total moves. The display follows it rather than fixing the first
  figure.

### Tests and docs

`internal/nixlog` 96.5%, `internal/ui` 90.9%. The decoder is tested by replaying
a committed capture from nix 2.34.8 (three chained derivations, so the moving
total is exercised; `testdata/capture.sh` regenerates it), plus an integration
test that runs a real build.

`docs/OUTPUT.md`'s verbosity table said Nix output is "suppressed" at `info`,
which is now wrong — it is consumed and re-reported. Corrected, with the moving
total and the raw-passthrough fallback documented, since both would otherwise
read as bugs.

The nix version map and CI matrix remain with `nixform2-lpqk`.
