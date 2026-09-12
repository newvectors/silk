#!/usr/bin/env bash
# Build the native amd64 EXE resource with the MSYS2 toolchain already used by CI.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
out=${1:-"$root/cmd/silkide/rsrc_windows_amd64.syso"}
windres=${WINDRES:-windres}
[[ -s "$root/assets/branding/silkide.ico" ]] || { echo 'Missing silkide.ico; run scripts/generate-branding-icons.sh first' >&2; exit 1; }
command -v "$windres" >/dev/null || { echo 'windres is required (MSYS2 UCRT64 binutils)' >&2; exit 1; }
mkdir -p "$(dirname "$out")"
"$windres" --target=pe-x86-64 --input-format=rc --output-format=coff \
  --include-dir "$root/assets/branding" \
  --input "$root/assets/branding/silkide.rc" --output "$out"
[[ -s "$out" ]] || { echo 'windres did not produce a Windows resource' >&2; exit 1; }
