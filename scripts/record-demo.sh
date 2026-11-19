#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

bash "$ROOT/scripts/demo-tutorial.sh" "$work" "path:${ROOT}"

fonts="$(nix build --no-link --print-out-paths nixpkgs#ibm-plex)"
cat >"$work/fonts.conf" <<CONF
<?xml version="1.0"?>
<!DOCTYPE fontconfig SYSTEM "fonts.dtd">
<fontconfig><dir>${fonts}/share/fonts</dir><cachedir>${work}/fontcache</cachedir></fontconfig>
CONF

mkdir -p "$work/tutorial/assets"
(
  cd "$work/tutorial"
  PATH="$work/bin:$PATH" FONTCONFIG_FILE="$work/fonts.conf" \
    nix shell nixpkgs#vhs -c vhs "$ROOT/assets/demo.tape"
)
cp "$work/tutorial/assets/demo.gif" "$ROOT/assets/demo.gif"
echo "recorded $ROOT/assets/demo.gif"
