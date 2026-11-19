#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

fail=0
bad() { echo "FAIL: $*"; fail=1; }

echo "== docs site: address, sharing tags, house style =="

book=docs-site/book.toml
head=docs-site/theme/head.hbs
css=docs-site/theme/custom.css

grep -qE '^site-url = "/"$' "$book" || bad "$book: site-url must be \"/\" (served from the root of docs.nivis.tf)"
grep -qE '^default-theme = "light"$' "$book" || bad "$book: default-theme must be \"light\""
grep -qE '^preferred-dark-theme = "navy"$' "$book" || bad "$book: preferred-dark-theme must be \"navy\""

hits="$(grep -rlF 'nivis-project.github.io' README.md docs-site --exclude-dir=book 2>/dev/null || true)"
[ -z "$hits" ] || bad "the project-page address nivis-project.github.io is still named in: $hits"

want_meta() {
  local attr="$1" name="$2" value="$3"
  grep -qF "<meta $attr=\"$name\" content=\"$value\">" "$head" || bad "$head: expected <meta $attr=\"$name\" content=\"$value\">"
}
want_meta property og:image https://nivis.tf/social.png
want_meta property og:image:type image/png
want_meta property og:image:width 1200
want_meta property og:image:height 630
want_meta name twitter:image https://nivis.tf/social.png
grep -qF '<link rel="icon" type="image/svg+xml" href="https://nivis.tf/favicon.svg">' "$head" || bad "$head: the favicon must be https://nivis.tf/favicon.svg"
if grep -oE '(href|content)="/[^"]*"' "$head" | grep -q .; then
  bad "$head: a site-relative URL in the head; sharing systems need absolute ones"
fi

grep -qF 'family=Hind:' "$css" || bad "$css: Hind is not loaded"
grep -qF 'family=IBM+Plex+Mono:' "$css" || bad "$css: IBM Plex Mono is not loaded"
grep -qE -- '--nv-body-font: "Hind"' "$css" || bad "$css: the body font is not Hind"
if grep -qiE 'cinzel|schibsted|ember|magma|glacier|--tn-' "$css"; then
  bad "$css: a family or colour of the previous brand remains: $(grep -oiE 'cinzel|schibsted|ember|magma|glacier|--tn-[a-z-]+' "$css" | sort -u | tr '\n' ' ')"
fi
if grep -qE '#[0-9a-fA-F]{3,8}\b' "$css"; then
  bad "$css: a hex colour; every colour is a nivis.tf token in oklch"
fi
for theme in light navy; do
  for token in ground surface ink muted line accent accent-soft; do
    awk -v t=".$theme {" -v k="--nv-$token:" '
      index($0, t) == 1 { inside = 1; next }
      inside && /^}/ { inside = 0 }
      inside && index($0, k) && /oklch\(/ { found = 1 }
      END { exit !found }' "$css" || bad "$css: .$theme does not define --nv-$token as an oklch colour"
  done
done

if command -v mdbook >/dev/null 2>&1; then
  out="$(mktemp -d)"
  trap 'rm -rf "$out"' EXIT
  if mdbook build docs-site -d "$out" >"$out.log" 2>&1; then
    grep -qF 'https://nivis.tf/social.png' "$out/index.html" || bad "the built index.html has no sharing image"
    grep -qE 'href="theme/custom(-[0-9a-f]+)?\.css"' "$out/index.html" || bad "the built index.html does not load theme/custom.css"
  else
    cat "$out.log"
    bad "mdbook build docs-site failed"
  fi
else
  echo "note: mdbook not installed, the build half of this check did not run"
fi

if [ "$fail" -ne 0 ]; then
  exit 1
fi
echo "docs site: ok"
