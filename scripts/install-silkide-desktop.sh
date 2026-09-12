#!/usr/bin/env bash
# Run explicitly after unpacking a Linux release. Installs only the current
# user's application entry and icon; no root, executable copies or network.
set -euo pipefail
app=$(cd "${1:-$(dirname "$0")}" && pwd)
[[ -x "$app/silkide" ]] || { echo 'Pass the unpacked release directory containing executable silkide' >&2; exit 1; }
[[ -s "$app/share/icons/hicolor/256x256/apps/silkide.png" ]] || { echo 'Missing packaged Silk IDE icon' >&2; exit 1; }
[[ "$app" != *$'\n'* && "$app" != *$'\r'* && "$app" != *$'\t'* && "$app" != *'='* ]] || { echo 'Control characters and equals signs are not supported in desktop installation paths' >&2; exit 1; }
data=${XDG_DATA_HOME:-"$HOME/.local/share"}
mkdir -p "$data/applications" "$data/icons/hicolor/256x256/apps"
cp "$app/share/icons/hicolor/256x256/apps/silkide.png" "$data/icons/hicolor/256x256/apps/silkide.png"
# Desktop Exec quoting is not shell quoting: preserve paths with spaces and
# escape field-code percent signs as well as the four reserved quoted chars.
# The desktop string layer then escapes every backslash once more:
# https://specifications.freedesktop.org/desktop-entry/latest/exec-variables.html
exe="$app/silkide"
exe=${exe//\\/\\\\}
exe=${exe//\"/\\\"}
exe=${exe//\`/\\\`}
exe=${exe//\$/\\\$}
exe=${exe//\\/\\\\}
exe=${exe//%/%%}
cat > "$data/applications/silkide.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=Silk IDE
Comment=Go code editor and visual interface designer
Exec="$exe"
Icon=silkide
Terminal=false
Categories=Development;IDE;
StartupNotify=true
EOF
echo "Installed $data/applications/silkide.desktop (keep the release at $app)"
