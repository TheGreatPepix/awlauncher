#!/bin/bash
# Packs the Linux build of the window and awcore into an AppImage.
# Usage: build-appimage.sh <bundle dir with awlauncher and awcore> <output file>
set -euo pipefail
bundle=$1
out=$2
here=$(cd "$(dirname "$0")" && pwd)
appdir=$(mktemp -d)/AWLauncher.AppDir
mkdir -p "$appdir"
cp -r "$bundle"/. "$appdir"/
cp "$here/AppRun" "$appdir/AppRun"
cp "$here/awlauncher.desktop" "$appdir/io.github.thegreatpepix.awlauncher.desktop"
cp "$here/awlauncher.png" "$appdir/io.github.thegreatpepix.awlauncher.png"
ln -s io.github.thegreatpepix.awlauncher.png "$appdir/.DirIcon"
tool=${APPIMAGETOOL:-appimagetool}
ARCH=x86_64 APPIMAGE_EXTRACT_AND_RUN=1 "$tool" --no-appstream "$appdir" "$out"
