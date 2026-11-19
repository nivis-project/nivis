#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
dir="${1:?usage: demo-tutorial.sh <empty-dir> [flake-ref]}"
ref="${2:-${ROOT}}"

mkdir -p "$dir/bin"
for out in $(nix build "${ref}#nivis" "${ref}#tutor" "${ref}#fake-providers" --no-link --print-out-paths); do
  ln -sf "$out"/bin/* "$dir/bin/"
done

PATH="$dir/bin:$PATH" nivistutor --tutorial getting-started --dir "$dir/tutorial" >/dev/null
sed -i -E "s|\"github:nivis-project/nivis[^\"]*\"|\"path:${ROOT}\"|" "$dir/tutorial/flake.nix"
if ! grep -qF "\"path:${ROOT}\"" "$dir/tutorial/flake.nix"; then
  echo "demo-tutorial: the scaffolded flake.nix does not point at this checkout" >&2
  exit 1
fi
