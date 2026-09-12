#!/usr/bin/env bash
# Rebuild desktop icon containers from the untouched generated PNG master.
# Requires ImageMagick 7 and macOS iconutil; neither is a runtime dependency.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
assets="$root/assets/branding"
source="$assets/silkide-logo.png"
profile=${SILKIDE_SRGB_PROFILE:-/System/Library/ColorSync/Profiles/sRGB Profile.icc}
for tool in magick iconutil; do
  command -v "$tool" >/dev/null 2>&1 || {
    echo "Missing icon-generation tool: $tool" >&2
    exit 1
  }
done
[[ -f "$source" ]] || { echo "Missing PNG master: $source" >&2; exit 1; }
[[ -f "$profile" ]] || { echo "Missing sRGB ICC profile: $profile" >&2; exit 1; }
read -r width height <<< "$(magick identify -format '%w %h' "$source")"
[[ "$width" == "$height" && "$width" -ge 1024 ]] || {
  echo 'PNG master must be square and at least 1024 pixels wide' >&2
  exit 1
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/SilkIDE.iconset"
sizes=(16 24 32 48 64 128 256 512 1024)
for size in "${sizes[@]}"; do
  # Preserve sRGB color and alpha; discard provenance metadata invalidated by
  # resizing. The original master, including its metadata, is never rewritten.
  magick "$source" -colorspace sRGB -filter Lanczos -resize "${size}x${size}" \
    +profile '*' -profile "$profile" \
    -define png:exclude-chunk=date,tIME,tEXt,zTXt,iTXt,eXIf,caBX \
    -depth 8 "PNG32:$tmp/silkide-$size.png"
done
magick "$tmp/silkide-16.png" "$tmp/silkide-24.png" \
  "$tmp/silkide-32.png" "$tmp/silkide-48.png" "$tmp/silkide-64.png" \
  "$tmp/silkide-128.png" "$tmp/silkide-256.png" "$tmp/silkide.ico"
for size in 16 32 128 256 512; do
  # iconutil's legacy 16/32px ARGB representations do not preserve the PNG's
  # RGB values on round-trip. Keep lossless PNG representations instead;
  # macOS can downscale these for small non-Retina slots.
  if [[ "$size" -ge 128 ]]; then
    cp "$tmp/silkide-$size.png" "$tmp/SilkIDE.iconset/icon_${size}x${size}.png"
  fi
  cp "$tmp/silkide-$((size * 2)).png" "$tmp/SilkIDE.iconset/icon_${size}x${size}@2x.png"
done
iconutil -c icns "$tmp/SilkIDE.iconset" -o "$tmp/silkide.icns"
for size in "${sizes[@]}"; do
  cp "$tmp/silkide-$size.png" "$assets/"
done
cp "$tmp/silkide.ico" "$tmp/silkide.icns" "$assets/"
echo 'Generated sRGB/alpha PNGs, seven-size Windows ICO and Retina macOS ICNS.'
