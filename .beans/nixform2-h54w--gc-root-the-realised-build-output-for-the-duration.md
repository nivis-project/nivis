---
# nixform2-h54w
title: GC-root the realised __build output for the duration of a run
status: todo
type: task
priority: normal
tags:
    - discovered
created_at: 2026-09-08T12:53:25Z
updated_at: 2026-09-08T12:53:25Z
parent: nixform2-kovh
---

Deferred from `nixform2-ebon` / OpenSpec change `realise-build-leaves-from-drv`, design Decision 5.

`nix-store --realise` warns when it is run without a GC root:

```
warning: you did not specify '--add-root'; the result might be removed by the garbage collector
```

`nivis` realises a `__build` output and then hands the path to a provider, which may spend minutes uploading it (a ~2 GB image). A concurrent `nix-collect-garbage` in that window can delete the path.

## Why it was not fixed in that change

The window existed before it too — but builds never happened, so it was effectively zero. Making builds real makes the window minutes long.

## The trade-off to decide

Adding `--add-root <path> --indirect` under a per-run temporary directory closes the window, but a crashed run then leaves a GC root pinning a large output until something cleans it. That is a deliberate trade (a leak instead of a disappearance), not a free win, which is why it wants its own change.

## Acceptance criteria

- A realised output cannot be collected while the run that realised it is still using it.
- A crashed run does not pin outputs indefinitely (roots live under a per-run directory that is removed, and a stale-root cleanup path is documented).
- Covered by a test that does not depend on running the garbage collector.
