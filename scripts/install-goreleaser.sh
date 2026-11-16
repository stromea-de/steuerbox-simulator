#!/usr/bin/env bash
# Upstream checksums retrieved 2026-11-16, GoReleaser OSS (MIT).
set -euo pipefail
case "$(uname -sm)" in
  'Darwin arm64') archive_arch=arm64; checksum=50bd83a6fcbf1dac272cfc4c46f9116052a9b0b463404a044af71a832fd92ad6 ;;
  'Darwin x86_64') archive_arch=x86_64; checksum=a1dfe9492c67b860129bc08fa5952955f0666abe4741e1846f93a6c6b47360e6 ;;
  *) echo 'macOS build host required' >&2; exit 1 ;;
esac
install_dir="${1:?Destination required}"
mkdir -p "$install_dir"
tool_tmp="$(mktemp -d)"
trap 'rm -rf "$tool_tmp"' EXIT
archive="goreleaser_Darwin_${archive_arch}.tar.gz"
curl --fail --location --retry 3 "https://github.com/goreleaser/goreleaser/releases/download/v2.20.0/$archive" -o "$tool_tmp/$archive"
(cd "$tool_tmp" && echo "$checksum  $archive" | shasum -a 256 -c -)
tar -xzf "$tool_tmp/$archive" -C "$tool_tmp" goreleaser
"$tool_tmp/goreleaser" --version | grep -Eq '^GitVersion: +2\.20\.0$'
cp "$tool_tmp/goreleaser" "$install_dir/goreleaser"
