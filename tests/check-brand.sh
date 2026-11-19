#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

fail=0
bad() { echo "FAIL: $*"; fail=1; }

echo "== brand: the nivis.tf house style =="

tagline="Terraform providers as first-class Nix values"
markers='volcanic|f2632e|242;99;46|cinzel|schibsted|all your base|infrastructure as nix code|nivis-emblem|nivis-glyph|banner\.(png|svg)|apple-touch-icon'

files=()
while IFS= read -r f; do
  [ -f "$f" ] || continue
  case "$f" in
    CHANGELOG.md | .beans/* | tests/check-brand.sh | tests/check-docs-site.sh) continue ;;
  esac
  files+=("$f")
done < <(git ls-files -co --exclude-standard)
[ "${#files[@]}" -gt 0 ] || bad "no tracked files to search"

hits="$(grep -IliE "$markers" "${files[@]}" 2>/dev/null || true)"
if [ -n "$hits" ]; then
  while IFS= read -r f; do
    bad "$f names the previous brand: $(grep -oiE "$markers" "$f" | sort -u | tr '\n' ' ')"
  done <<<"$hits"
fi

first_img="$(grep -oE '<img [^>]*src="[^"]*"' README.md | head -n1 | sed -E 's/.*src="([^"]*)"/\1/')"
[ "$first_img" = "https://nivis.tf/social.png" ] || bad "README.md: the first image is '$first_img', not https://nivis.tf/social.png"
grep -qF 'Formerly `nixform`, then Terrae Nivis' README.md || bad "README.md: the lineage note (formerly nixform, then Terrae Nivis) is missing"

for f in README.md docs-site/src/index.md docs-site/book.toml; do
  grep -qF "$tagline" "$f" || bad "$f does not carry the tagline \"$tagline\""
done
grep -qF "const tagline = \"$tagline\"" cmd/nivis/splash.go || bad "cmd/nivis/splash.go: the tagline constant is not \"$tagline\""
grep -qF '"Nivis: " + tagline' cmd/nivis/main.go || bad "cmd/nivis/main.go: the help text does not use the tagline"

caps=internal/ui/capability.go
for name in Accent Name Dim; do
  code="$(grep -E "^[[:space:]]+$name[[:space:]]+= " "$caps" | grep -oE '38;2;[0-9]+;[0-9]+;[0-9]+' || true)"
  if [ -z "$code" ]; then
    bad "$caps defines no truecolor $name"
    continue
  fi
  rgb="$(printf '%s' "$code" | cut -d';' -f3- | sed 's/;/, /g')"
  grep -qE "^\| \`$name\` +\| $rgb +\|" docs/BRAND.md || bad "docs/BRAND.md does not record $name as $rgb"
  stray="$(grep -rlF "$code" --include='*.go' cmd internal | grep -vE '_test\.go$' | grep -vxF "$caps" || true)"
  [ -z "$stray" ] || bad "$name is defined again outside $caps: $stray"
done

grep -qE '^[[:space:]]+Bright[[:space:]]+= "\\x1b\[1m"$' "$caps" || bad "$caps: Bright must be bold in the terminal's own colour, readable on light and dark"

if [ "$fail" -ne 0 ]; then
  exit 1
fi
echo "brand: ok"
