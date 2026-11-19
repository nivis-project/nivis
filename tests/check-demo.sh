#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

fail=0
bad() { echo "   FAIL: $*"; fail=1; }

tape=assets/demo.tape
gif=assets/demo.gif

[ -f "$tape" ] || { echo "   FAIL: $tape is missing"; exit 1; }
[ -f "$gif" ] || { echo "   FAIL: $gif is missing"; exit 1; }

grep -qE '^Output assets/demo\.gif$' "$tape" || bad "$tape does not write $gif"
grep -qF '"background": "#020713"' "$tape" || bad "$tape: the background is not code-bg (dark), #020713"
grep -qF '"foreground": "#deeaf0"' "$tape" || bad "$tape: the foreground is not code-ink, #deeaf0"

seconds="$(python3 - "$gif" 2>&1 <<'PY'
import sys
d = open(sys.argv[1], "rb").read()
if d[:6] not in (b"GIF87a", b"GIF89a"):
    sys.exit("not a GIF")
i = 13
if d[10] & 0x80:
    i += 3 << ((d[10] & 7) + 1)
total = 0
while i < len(d):
    b = d[i]
    if b == 0x3B:
        break
    if b == 0x21:
        label = d[i + 1]
        j = i + 2
        if label == 0xF9:
            total += int.from_bytes(d[j + 2:j + 4], "little")
        while d[j]:
            j += d[j] + 1
        i = j + 1
    elif b == 0x2C:
        flags = d[i + 9]
        j = i + 10
        if flags & 0x80:
            j += 3 << ((flags & 7) + 1)
        j += 1
        while d[j]:
            j += d[j] + 1
        i = j + 1
    else:
        sys.exit(f"unexpected block 0x{b:02x} at {i}")
print(f"{total / 100:.2f}")
PY
)" || bad "$gif could not be read: $seconds"
if [ "$fail" -eq 0 ]; then
  python3 -c "import sys; sys.exit(0 if 10 <= float('$seconds') <= 20 else 1)" || bad "$gif lasts ${seconds}s; between 10 and 20 is the range, and less means the recording dropped frames"
fi

commands="$(awk '
  /^Hide$/ { hidden = 1; next }
  /^Show$/ { hidden = 0; next }
  !hidden && /^Type "/ { sub(/^Type "/, ""); sub(/"$/, ""); print }
' "$tape")"
want="$(printf 'nivis plan\nnivis apply\nnivis output')"
[ "$commands" = "$want" ] || bad "$tape types $(echo "$commands" | tr '\n' ',') where the checked sequence is plan, apply, output"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
bash scripts/demo-tutorial.sh "$work" >/dev/null
out=()
while IFS= read -r cmd; do
  [ -n "$cmd" ] || continue
  out+=("$(cd "$work/tutorial" && PATH="$work/bin:$PATH" NO_COLOR=1 $cmd 2>&1)")
done <<<"$commands"

plan="${out[0]:-}"
apply="${out[1]:-}"
output="${out[2]:-}"
[ "$(printf '%s\n' "$plan" | grep -cE '^  \+ ')" -eq 3 ] || bad "nivis plan does not list three creations:
$plan"
printf '%s\n' "$apply" | grep -qF 'Applied 3 resource(s) across 3 phase(s)' || bad "nivis apply did not reach the fixpoint in three phases:
$apply"
printf '%s\n' "$output" | grep -qxF 'combined = beta://rec-alpha::0::alpha::0' || bad "nivis output does not show the combined value:
$output"

if [ "$fail" -ne 0 ]; then
  exit 1
fi
echo "   ok: the tape types plan, apply, output; they show 3 creations, 3 phases and the combined value; ${seconds}s"
