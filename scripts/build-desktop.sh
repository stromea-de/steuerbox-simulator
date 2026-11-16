#!/usr/bin/env bash
# Build locally. Windows uses pure Go + GUI subsystem; macOS Cocoa needs CGO.
set -euo pipefail
cd "$(dirname "$0")/.."
target_os="${1:-windows}"
target_arch="${2:-amd64}"
case "$target_os/$target_arch" in windows/amd64|windows/arm64|darwin/amd64|darwin/arm64|linux/amd64|linux/arm64) ;; *) echo 'Ziel: windows|darwin|linux amd64|arm64' >&2; exit 2 ;; esac
build_version="$(git describe --tags --exact-match 2>/dev/null || true)"
build_commit="$(python3 scripts/build-id.py)"
build_flags="-s -w -X github.com/stromea-de/steuerbox-simulator/internal/report.Version=$build_version -X github.com/stromea-de/steuerbox-simulator/internal/report.Commit=$build_commit"
build_cgo=0
build_suffix=''
if [[ "$target_os" == windows ]]; then build_flags="-H=windowsgui $build_flags"; build_suffix=.exe; fi
if [[ "$target_os" == darwin ]]; then
  if [[ "$(uname -s)" != Darwin ]]; then echo 'macOS-Menü benötigt einen macOS-Buildhost mit Xcode Command Line Tools.' >&2; exit 1; fi
  build_cgo=1
fi
mkdir -p bin
CGO_ENABLED="$build_cgo" GOOS="$target_os" GOARCH="$target_arch" go build -trimpath -ldflags "$build_flags" -o "bin/stromea-steuerbox-simulator-$target_os-$target_arch$build_suffix" ./cmd/steuerbox-simulator
