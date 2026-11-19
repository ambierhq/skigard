#!/usr/bin/env bash
# Rebuild every demo asset: recordings (VHS), GIF/MP4/WebM, poster and screenshots.
# Needs: go (via mise), vhs, ttyd, ffmpeg, and Chrome/Chromium for the poster (optional: set CHROME=/path).
# Outputs land in assets/media/. Intermediate files go to a temp dir. Never calls `approve` outside a pty:
# scene 3 types it into VHS's pseudo-terminal, scene 4 uses scripts/pty_approve.py.
set -euo pipefail
cd "$(dirname "$0")/.."
ROOT=$PWD
for t in vhs ttyd ffmpeg python3; do
  command -v "$t" >/dev/null || { echo "demo: missing $t (brew install vhs ttyd ffmpeg)" >&2; exit 2; }
done
mise run build >/dev/null 2>&1 || go build -o dist/skigard ./cmd/skigard

export DEMO_ROOT=$ROOT DEMO_BIN=$ROOT/dist DEMO_TMP=${DEMO_TMP:-/tmp/skigard-demo}
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
rm -rf "$DEMO_TMP"; mkdir -p "$DEMO_TMP" "$ROOT/assets/media"

# 1. record each scene (theme + tape are concatenated because Output/Screenshot paths are relative to the cwd)
scenes=()
for tape in "$ROOT"/assets/demo/scene*.tape; do
  base=$(basename "$tape" .tape)
  cat "$ROOT/assets/demo/lib/theme.tape" "$tape" > "$WORK/$base.tape"
  (cd "$WORK" && vhs "$base.tape" >/dev/null)
  scenes+=("$WORK/$(sed -n 's/^Output "\(.*\)"/\1/p' "$tape")")
done

# 2. stitch scenes and encode
: > "$WORK/list.txt"
for s in "${scenes[@]}"; do
  echo "file '$s'" >> "$WORK/list.txt"
  echo "scene $(basename "$s"): $(ffprobe -v error -show_entries format=duration -of csv=p=0 "$s")s (budget 30s)"
done
ffmpeg -y -loglevel error -f concat -safe 0 -i "$WORK/list.txt" -c copy "$WORK/all.mp4"
OUT=$ROOT/assets/media
ffmpeg -y -loglevel error -i "$WORK/all.mp4" -an -vf "scale=1120:-2,format=yuv420p" \
  -c:v libx264 -preset veryslow -crf 30 -movflags +faststart "$OUT/demo.mp4"
ffmpeg -y -loglevel error -i "$WORK/all.mp4" -an -vf "scale=1120:-2" \
  -c:v libvpx-vp9 -crf 38 -b:v 0 -row-mt 1 "$OUT/demo.webm"
ffmpeg -y -loglevel error -i "$WORK/all.mp4" \
  -vf "fps=12,scale=960:-1:flags=lanczos,split[a][b];[a]palettegen=max_colors=64:stats_mode=diff[p];[b][p]paletteuse=dither=none" \
  "$OUT/demo.gif"

if [ -n "${DEMO_KEEP:-}" ]; then mkdir -p "$DEMO_KEEP"; cp "${scenes[@]}" "$DEMO_KEEP/"; fi

# 3. screenshots (consistent: same theme, window chrome and size)
cp "$WORK/shot-1-block.png"    "$OUT/screenshot-1-block.png"
cp "$WORK/shot-2-refused.png"  "$OUT/screenshot-2-agent-refused.png"
cp "$WORK/shot-3-approved.png" "$OUT/screenshot-3-approve.png"
cp "$WORK/shot-4-tamper.png"   "$OUT/screenshot-4-verify-fails.png"

# 4. poster (brand card; the mark is read from assets/brand/)
CHROME=${CHROME:-}
if [ -z "$CHROME" ]; then
  for c in "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" google-chrome chromium chromium-browser; do
    if command -v "$c" >/dev/null 2>&1 || [ -x "$c" ]; then CHROME=$c; break; fi
  done
fi
if [ -n "$CHROME" ]; then
  "$CHROME" --headless --disable-gpu --hide-scrollbars --window-size=1280,720 \
    --screenshot="$OUT/poster.png" "file://$ROOT/assets/demo/poster.html" >/dev/null 2>&1
else
  echo "demo: no Chrome found, using a video frame as poster (set CHROME=...)" >&2
  ffmpeg -y -loglevel error -ss 6 -i "$OUT/demo.mp4" -frames:v 1 "$OUT/poster.png"
fi

# 5. budgets from the playbook: GIF < 5 MB, each site video < 3 MB
fail=0
for f in demo.gif:5242880 demo.mp4:3145728 demo.webm:3145728; do
  n=${f%%:*}; max=${f##*:}; sz=$(wc -c < "$OUT/$n")
  echo "$n $sz bytes"; [ "$sz" -lt "$max" ] || { echo "demo: $n over budget" >&2; fail=1; }
done
ls -l "$OUT"
exit $fail
