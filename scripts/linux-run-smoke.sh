#!/usr/bin/env bash
# A virtual-display startup check, not an interactive/visual desktop test.
# Isolate preferences/workspace so CI or local user state cannot affect it.
# bash -c and its EXIT trap intentionally expand variables in the child shell.
# shellcheck disable=SC2016
set -euo pipefail

bin=${1:?Usage: linux-run-smoke.sh BINARY [SECONDS]}
seconds=${2:-12}
if [[ $(uname -s) != Linux || ! -x $bin || ! $seconds =~ ^[1-9][0-9]*$ ]]; then
  echo 'Linux, an executable binary and positive integer seconds are required' >&2
  exit 2
fi
for tool in xvfb-run xwininfo go; do
  command -v "$tool" >/dev/null || { echo "Missing smoke-test tool: $tool" >&2; exit 2; }
done
bin=$(cd "$(dirname "$bin")" && pwd)/$(basename "$bin")
resources=$(cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/home" "$tmp/workspace"
printf 'module silk-smoke\n\ngo 1.26\n' > "$tmp/workspace/go.mod"
printf 'package main\nfunc main() {}\n' > "$tmp/workspace/main.go"

HOME="$tmp/home" XDG_CONFIG_HOME="$tmp/home/.config" \
SILK_DATA_DIR="$tmp/home/data" SILK_WORKSPACE_DIR="$tmp/workspace" \
SILK_RESOURCE_DIR="$resources" LIBGL_ALWAYS_SOFTWARE=1 \
xvfb-run -a -s '-screen 0 1280x800x24' bash -c '
  set -euo pipefail
  bin=$1; seconds=$2; tmp=$3
  cd "$tmp/workspace"
  "$bin" > "$tmp/app.log" 2>&1 &
  pid=$!
  trap '\''kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true'\'' EXIT
  for ((n=0; n<seconds; n++)); do
    sleep 1
    if ! kill -0 "$pid" 2>/dev/null; then
      cat "$tmp/app.log"
      echo "FAIL: IDE exited before ${seconds}s" >&2
      exit 1
    fi
  done
  xwininfo -root -tree > "$tmp/windows.log"
  cat "$tmp/windows.log"
  cat "$tmp/app.log"
  grep -q "GLFW-Application" "$tmp/windows.log" || {
    echo "FAIL: no native GLFW window appeared" >&2; exit 1;
  }
  if grep -qiE "panic:|fatal error|checkptr|main loop mechanism unavailable|go list .*failed" "$tmp/app.log"; then
    echo "FAIL: fatal startup or package-discovery marker" >&2
    exit 1
  fi
  echo "PASS: Linux Xvfb native window survived ${seconds}s; no fatal/package-discovery errors"
' smoke "$bin" "$seconds" "$tmp"
