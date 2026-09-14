# Reading Nivis output

A `nivis` run spends most of its wall-clock time inside three slow things: a Nix
evaluation of your configuration, a Nix build of whatever artifacts it
references, and provider calls that wait on real infrastructure. This page
describes what Nivis tells you while that is happening, where each kind of
output goes, and how to turn the detail up or down.

## The three channels

Nivis divides its output into three kinds of content, each with a fixed
destination. The split is what lets a script and a human share one command.

| Channel | Where | What it carries |
|--------------------|-----------------------|-------------------------------------|
| **The result** | stdout | The change list and the final summary |
| **The narrative** | stderr | Progress, provider notes, Nix output |
| **The live region**| stderr, terminal only | A status display, rewritten in place |

**The result** is what the command concluded. It is byte-identical whether or
not a terminal is attached, so this keeps working exactly as before:

```console
$ nivis apply > result.txt      # the same bytes a pipeline would see
$ nivis plan | grep '^\s*+'     # stable enough to parse
```

**The narrative** is what happened along the way. It is durable — redirect it and
you get a full transcript, differing from the on-screen form only in carrying no
colour:

```console
$ nivis apply 2> run.log
```

**The live region** is the small block of status lines at the bottom of the
screen during a run, rewritten in place and erased before the run ends. It never
appears in redirected output, so a log file never contains half-drawn spinner
frames.

The rule dividing the first two: content that reports **what happened** is
narrative, and content that reports **what resulted** is the result. Acquiring
the state lock, for instance, is narrative.

> **Note**
> A node you watch complete in the narrative also appears in the result's
> phase-grouped recap at the end. That is deliberate — they are different views.
> The narrative is chronological and carries durations; the recap is grouped by
> phase and carries change markers.

## Verbosity

`--log-level` controls how much of Nivis's **own** narrative you see. It can also
be set once with the `NIVIS_LOG` environment variable; an explicit flag wins.

| Level | What you see |
|-----------|------------------------------------------------------------------|
| `quiet` | The result and errors, nothing else |
| `info` | **Default.** Each unit of work as it completes, phase boundaries, and a live region where the terminal allows |
| `verbose` | Also: work as it *starts*, evaluation and provider-startup timings, every refreshed resource named, and Nix's own output passed through |
| `debug` | Also: detail for diagnosing Nivis itself |

```console
$ nivis apply --log-level verbose
$ NIVIS_LOG=quiet nivis plan
```

### The default reports what changed, not what happened

At `info`, a bulk operation reports its **outcome** rather than narrating every
step. A `plan` refreshes every resource in state through its provider; naming all
forty buries the three that matter, so the default names only the drifted ones
and counts the rest. `--log-level verbose` names every read, when you want it.

### `--log-level` is not `--provider-log-level`

They govern different things and are independently settable:

- `--log-level` — how much **Nivis** says about its own execution.
- `--provider-log-level` — how much of a **spawned provider's** logging is
  surfaced as notes. See [Getting started](./GETTING-STARTED.md).

Silencing one does not silence the other. `--log-level quiet` still shows a
provider's deprecation warning.

## Nix's own output

Nix has a perfectly good progress display of its own, and a build of an
operating-system image can take many minutes. But Nix's display and Nivis's live
region both drive the cursor, so they can never both be active. Exactly one is,
chosen by the verbosity:

| `--log-level` | Nix's own output | Nivis's live region |
|---------------|----------------------|---------------------|
| `quiet` | suppressed | off |
| `info` | suppressed | **on** |
| `verbose` | passed through raw | off |
| `debug` | passed through raw | off |

So when you want to watch a long build the way Nix reports it:

```console
$ nivis apply --log-level verbose
```

At the default level you still see that the build started, what is building, and
how long it has been going — just in Nivis's own form rather than Nix's.

Whichever level you choose, a failing evaluation or build always reports its
actionable error text.

## Colour

`--color` accepts `auto` (the default), `always` and `never`.

- `auto` colourises a colour-capable terminal, unless colour is disabled by the
  environment.
- `always` colourises even when output is redirected — useful for a CI system
  that renders ANSI in its log viewer.
- `never` never colourises.

[`NO_COLOR`](https://no-color.org) disables colour when set to anything;
`CLICOLOR_FORCE` enables it, as its counterpart. An explicit `--color` wins over
both. A terminal reporting itself as `dumb` is never treated as colour-capable.

Colour changes presentation only. The markers, the text and the counts are
identical either way, so a test or a script sees stable output.

### Colour and the live region are separate questions

Turning colour off does **not** turn the live region off, and forcing colour on
does not turn it on. A terminal can render colour correctly and still mishandle
cursor movement, so Nivis asks the two questions separately — conflating them
corrupts output on exactly the terminals least able to cope.

The live region is used only when all of these hold: output is a terminal, `TERM`
is not `dumb`, no CI environment is detected, and the level is `info`. Otherwise
you get one line per transition carrying the same information.

## Reading a phased apply

Nivis resolves your configuration over **phases**. A phase boundary means the
nodes in the next phase could not have run any earlier — their inputs only became
known once the previous phase's outputs were fed back into Nix and it was
re-evaluated. That round trip is the thing Nivis exists to do, and the phase
headings are where you watch it happen.

```
Phase 1  3 nodes
  + alpha.alpha_token.app        1s   alpha-0
  r data.alpha.alpha_lookup.app  0ms
Phase 2  1 node
  + beta.beta_record.dns        42s   rec-17
```

Each phase announces how many nodes are ready in it *before* running them. There
is deliberately no total for the whole run: Nivis cannot know it in advance,
because a later phase can reveal resources that phase 0 could not see. A count
that looked authoritative would be a guess.

The markers are the same ones `plan` uses: `+` create, `~` update, `-/+` replace,
`=` no change, `r` a datasource read.
