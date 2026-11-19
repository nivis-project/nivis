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
if grep -qE 'rel="(shortcut )?icon"' "$head"; then
  bad "$head: an icon link; mdBook links theme/favicon.svg itself, and a second link loses to it"
fi
mark=docs-site/theme/favicon.svg
mark_sha256=e64fce8e86010fe6add5d89876149afd4fa5d0e3eac85e416bf98da658aa9d0e
[ "$(sha256sum "$mark" | cut -d' ' -f1)" = "$mark_sha256" ] || bad "$mark is not the nivis.tf mark (https://nivis.tf/favicon.svg, sha256 $mark_sha256)"
[ ! -e docs-site/theme/favicon.png ] || bad "docs-site/theme/favicon.png exists; mdBook would link it as a second icon"
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
    pages=0
    while IFS= read -r page; do
      pages=$((pages + 1))
      icons="$(grep -oE '<link rel="(shortcut )?icon"[^>]*>' "$page" || true)"
      n="$(printf '%s' "$icons" | grep -c . || true)"
      if [ "$n" -ne 1 ]; then
        bad "$page has $n icon links, expected 1"
        continue
      fi
      href="$(printf '%s' "$icons" | sed -E 's/.*href="([^"]*)".*/\1/')"
      case "$href" in
        /*) file="$out$href" ;;
        *) file="$(dirname "$page")/$href" ;;
      esac
      cmp -s "$file" "$mark" || bad "$page: its icon $href is not $mark"
    done < <(find "$out" -name '*.html' ! -name 'toc.html')
    [ "$pages" -gt 0 ] || bad "the build produced no pages to inspect"
    first_img="$(grep -oE '<img [^>]*src="[^"]*"' "$out/index.html" | head -n1 | sed -E 's/.*src="([^"]*)"/\1/')"
    [ "$first_img" = "https://nivis.tf/social.png" ] || bad "the first image on the landing page is '$first_img', not https://nivis.tf/social.png"
    hits="$(grep -rlE 'src="[^"]*banner\.png"' "$out" --include='*.html' || true)"
    [ -z "$hits" ] || bad "a built page still loads banner.png: $hits"
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
