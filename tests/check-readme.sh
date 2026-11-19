#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

fail=0
bad() { echo "FAIL: $*"; fail=1; }

echo "== README: opening, sections, audiences, comparison, feedback =="

readme=README.md
repo=https://github.com/nivis-project/nivis

line_of() { grep -nF -- "$1" "$readme" | head -n1 | cut -d: -f1 || true; }

first_section="$(grep -nE '^## ' "$readme" | head -n1 | cut -d: -f1 || true)"
[ -n "$first_section" ] || { echo "FAIL: $readme has no sections"; exit 1; }

prev=0
for marker in \
  'src="https://nivis.tf/social.png"' \
  'img.shields.io/github/v/release/nivis-project/nivis' \
  'img.shields.io/badge/license-Apache--2.0' \
  '/actions/workflows/ci.yml/badge.svg' \
  'built%20with-Nix' \
  '**Terraform providers as first-class Nix values.**' \
  'src="assets/demo.gif"' \
  '**Status: early but real.**' \
  '[Website](https://nivis.tf)' \
  '[Docs](https://docs.nivis.tf/)' \
  '[Registry](https://registry.nivis.tf)' \
  "[Discussions]($repo/discussions)"; do
  n="$(line_of "$marker")"
  if [ -z "$n" ]; then
    bad "the opening lacks $marker"
  elif [ "$n" -ge "$first_section" ]; then
    bad "$marker is not in the opening, before the first section"
  elif [ "$n" -lt "$prev" ]; then
    bad "$marker is out of order in the opening"
  else
    prev="$n"
  fi
done

[ -f assets/demo.gif ] || bad "the README shows assets/demo.gif, which does not exist"
for wf in $(grep -oE 'actions/workflows/[a-z0-9_-]+\.yml/badge\.svg' "$readme" | cut -d/ -f3); do
  [ -f ".github/workflows/$wf" ] || bad "the CI badge names .github/workflows/$wf, which does not exist"
done
grep -qF 'Apache License' LICENSE && grep -qF 'Version 2.0' LICENSE || bad "the licence badge says Apache-2.0 but LICENSE is not the Apache License 2.0"

want="$(printf '%s\n' \
  'You already know half of it' 'Quickstart' 'How it works' 'The Nix library' \
  'Stable contracts' 'How it compares' 'Questions & feedback' 'Contributing' 'License')"
got="$(grep -E '^## ' "$readme" | sed 's/^## //' || true)"
[ "$got" = "$want" ] || bad "the sections are
$got
expected
$want"

section() { awk -v h="## $1" '$0 == h { p = 1; next } p && /^## / { exit } p' "$readme"; }

audiences="$(section 'You already know half of it')"
printf '%s\n' "$audiences" | grep -qF '**Coming from Nix?**' || bad "'You already know half of it' does not address a reader coming from Nix"
printf '%s\n' "$audiences" | grep -qF '**Coming from Terraform?**' || bad "'You already know half of it' does not address a reader coming from Terraform"
printf '%s\n' "$audiences" | awk '/^```hcl$/ { p = 1 } p && /^```$/ { p = 0 } p' | grep -qF 'resource "aws_s3_bucket" "demo"' || bad "'You already know half of it' has no hcl block with the S3 bucket"
printf '%s\n' "$audiences" | awk '/^```nix$/ { p = 1 } p && /^```$/ { p = 0 } p' | grep -qF 'type = "aws_s3_bucket"' || bad "'You already know half of it' has no nix block with the S3 bucket"

quick="$(section 'Quickstart')"
tour="$(printf '%s\n' "$quick" | grep -nF 'nivistutor' | head -n1 | cut -d: -f1 || true)"
apply="$(printf '%s\n' "$quick" | grep -nE '^nivis apply' | head -n1 | cut -d: -f1 || true)"
if [ -z "$tour" ]; then
  bad "the Quickstart does not offer the nivistutor tour"
elif [ -n "$apply" ] && [ "$tour" -gt "$apply" ]; then
  bad "the Quickstart reaches nivis apply before the nivistutor tour"
fi

note='The round trip is not unique to Nivis'
compare="$(section 'How it compares')"
printf '%s\n' "$compare" | grep -qF "$note" || bad "'How it compares' does not hold the note \"$note\""
printf '%s\n' "$compare" | grep -qF '(docs/COMPARISON.md)' || bad "'How it compares' does not link docs/COMPARISON.md"
n="$(line_of "$note")"
[ -z "$n" ] || [ "$n" -gt "$first_section" ] || bad "\"$note\" is in the opening; it belongs under 'How it compares'"

feedback="$(section 'Questions & feedback')"
printf '%s\n' "$feedback" | grep -qF "($repo/discussions)" || bad "'Questions & feedback' does not link Discussions"
printf '%s\n' "$feedback" | grep -qF "($repo/issues)" || bad "'Questions & feedback' does not link Issues"

if [ "$fail" -ne 0 ]; then
  exit 1
fi
echo "README: ok"
