# Tasks: shell-completion

## 1. Completer
- [x] 1.1 `cmd/nivis`: `stateIDs(cmd, args, toComplete) ([]string, ShellCompDirective)`:
      open the store, `List()`, return the ids; on any error return `nil,
      ShellCompDirectiveNoFileComp` (no suggestions, no filename fallback).
- [x] 1.2 Wire it as `ValidArgsFunction` on `state show` and `state rm` (only for
      the first arg; return NoFileComp for further args).
- [x] 1.3 Register it for the `--target` persistent flag via
      `root.RegisterFlagCompletionFunc("target", stateIDs)`.
- [x] 1.4 Confirm Cobra's default `completion` command is present (not disabled);
      keep it.

## 2. Tests
- [x] 2.1 `cmd/nivis`: a populated store (write a couple of ResourceStates to a
      temp state file, point `--state`/openStore at it) -> stateIDs returns those
      ids.
- [x] 2.2 A missing/empty state file -> stateIDs returns no ids and the
      NoFileComp directive.

## 3. Docs (docs-coverage gate: new section)
- [x] 3.1 `docs/INSTALL.md`: an "Shell completion" section with the per-shell
      one-liners (bash/zsh/fish). No em dashes.

## 4. Gate
- [x] 4.1 `gofmt`, `go build ./...`, `go test ./...` green.
- [x] 4.2 `bash tests/check-docs-ssot.sh` green (INSTALL.md touched).
- [x] 4.3 `openspec validate shell-completion --strict`; archive; close beans-oycy.
