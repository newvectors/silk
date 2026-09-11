#!/usr/bin/env bash
# Debian/Ubuntu prerequisites for the Cairo + GLFW/X11 IDE backend.
# --print is side-effect free and is also used by packaging regression tests.
set -euo pipefail

packages=(
  build-essential pkg-config libcairo2-dev
  libx11-dev libxcursor-dev libxi-dev libxinerama-dev libxrandr-dev
  libxxf86vm-dev libgl1-mesa-dev
  zenity xdg-utils
)

if [[ ${1:-} == --print ]]; then
  printf '%s\n' "${packages[@]}"
  exit 0
fi
if [[ $# != 0 || $(uname -s) != Linux ]] || ! command -v apt-get >/dev/null; then
  echo 'Usage: install-linux-deps.sh [--print] (Debian/Ubuntu; other distributions: see README.md)' >&2
  exit 2
fi

sudo_cmd=()
if [[ $EUID != 0 ]]; then
  sudo_cmd=(sudo)
fi
"${sudo_cmd[@]}" apt-get update
"${sudo_cmd[@]}" apt-get install -y --no-install-recommends "${packages[@]}"
