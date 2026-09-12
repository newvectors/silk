#!/usr/bin/env bash
# Add native launcher identity to an already staged release directory.
# Does not bundle/relocate Cairo or alter the independently shipped SDK tools.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
platform=${1:?usage: package-silkide-branding.sh linux|darwin|windows STAGING_DIR [IDE_VERSION]}
out=${2:?missing staging directory}
version=${3:-$(awk -F '"' '/^const ideVersion = / { print $2; exit }' "$root/cmd/silkide/menu_actions.go")}
[[ "$version" =~ ^[0-9]+(\.[0-9]+){0,3}$ ]] || { echo 'IDE version must contain 1-4 numeric components' >&2; exit 1; }
assets=${SILKIDE_BRANDING_DIR:-"$root/assets/branding"}
for name in silkide-256.png silkide.ico silkide.icns; do
  [[ -s "$assets/$name" ]] || { echo "Missing branding asset: $name" >&2; exit 1; }
done
case "$platform" in
  darwin)
    [[ -f "$out/silkide" ]] || { echo 'Missing staged silkide executable' >&2; exit 1; }
    app="$out/SilkIDE.app/Contents"
    mkdir -p "$app/MacOS" "$app/Resources"
    cp "$out/silkide" "$app/MacOS/silkide"
    chmod +x "$app/MacOS/silkide"
    cp "$assets/silkide.icns" "$app/Resources/silkide.icns"
    if [[ -d "$out/icon" ]]; then cp -R "$out/icon" "$app/Resources/"; fi
    cat > "$app/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>org.silkui.silkide</string>
<key>CFBundleName</key><string>Silk IDE</string>
<key>CFBundleDisplayName</key><string>Silk IDE</string>
<key>CFBundleExecutable</key><string>silkide</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleIconFile</key><string>silkide.icns</string>
<key>CFBundleShortVersionString</key><string>$version</string>
<key>CFBundleVersion</key><string>$version</string>
<key>NSHighResolutionCapable</key><true/>
</dict></plist>
EOF
    ;;
  linux)
    [[ -f "$out/silkide" ]] || { echo 'Missing staged silkide executable' >&2; exit 1; }
    mkdir -p "$out/share/icons/hicolor/256x256/apps"
    cp "$assets/silkide-256.png" "$out/share/icons/hicolor/256x256/apps/silkide.png"
    cp "$root/scripts/install-silkide-desktop.sh" "$out/install-desktop-entry.sh"
    chmod +x "$out/install-desktop-entry.sh"
    ;;
  windows)
    [[ -f "$out/silkide.exe" ]] || { echo 'Missing staged silkide.exe' >&2; exit 1; }
    # Explorer gets its primary icon from the embedded EXE resource; this file
    # is additionally useful when a user configures an existing shortcut.
    cp "$assets/silkide.ico" "$out/silkide.ico"
    ;;
  *) echo "Unsupported branding platform: $platform" >&2; exit 1 ;;
esac
