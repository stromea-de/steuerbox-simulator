#!/usr/bin/env bash
# Prepare a portable AppDir. Standard AppImage runtime is excluded by license policy.
set -euo pipefail
cd "$(dirname "$0")/.."
arch="${1:-}"
version="${2:-}"
if [[ "$arch" != amd64 && "$arch" != arm64 ]] || [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?$ ]] || [[ $# -gt 2 ]]; then
  echo 'Aufruf: scripts/appimage.sh <amd64|arm64> <Version>' >&2
  exit 2
fi
archive="dist/stromea-steuerbox-simulator_${version}_linux_${arch}.tar.gz"
test -f "$archive"
appdir="dist/stromea-steuerbox-simulator-${arch}.AppDir"
mkdir -p "$appdir/usr/bin" "$appdir/usr/share/doc/stromea-steuerbox-simulator"
tar -xzf "$archive" -C "$appdir/usr/share/doc/stromea-steuerbox-simulator"
mv "$appdir/usr/share/doc/stromea-steuerbox-simulator/stromea-steuerbox-simulator" "$appdir/usr/bin/stromea-steuerbox-simulator"
cp packaging/appimage/AppRun packaging/appimage/stromea-steuerbox-simulator.desktop packaging/appimage/stromea-steuerbox-simulator.svg "$appdir/"
chmod +x "$appdir/AppRun" "$appdir/usr/bin/stromea-steuerbox-simulator"
echo "AppDir vorbereitet: $appdir"
echo 'Kein .AppImage erzeugt: Standardruntime enthält LGPL-libfuse; nur permissive Abhängigkeiten erlaubt. Siehe docs/VERTEILUNG.md.' >&2
