#!/usr/bin/env bash
# Build only. No signing keys, attestation, release creation or network to devices.
set -euo pipefail
cd "$(dirname "$0")/.."
mode="${1:-tag}"
if [[ $# -gt 1 || ( "$mode" != tag && "$mode" != snapshot ) ]]; then
  echo 'Usage: scripts/release-ci.sh [tag|snapshot]' >&2; exit 2
fi
if [[ -n "$(git status --porcelain)" ]]; then
  echo 'CI-Build verlangt sauberen Checkout.' >&2; exit 1
fi
if [[ "$mode" == tag ]]; then
  release_tag="$(git describe --tags --exact-match HEAD 2>/dev/null || true)"
  if [[ ! "$release_tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo 'Exakter stabiler Tag vX.Y.Z erforderlich.' >&2; exit 1
  fi
  release_version="${release_tag#v}"
else
  release_version=1.0.0
fi
if [[ "$(uname -s)" != Darwin ]]; then echo 'macOS + Xcode Command Line Tools required.' >&2; exit 1; fi
[[ "$(go env GOVERSION)" == go1.27.3 ]]
goreleaser --version | grep -Eq '^GitVersion: +2\.20\.0$'
export STROMEA_REPORT_VERSION="$release_version"
export STROMEA_REPORT_COMMIT="$(python3 scripts/build-id.py)"
[[ "$STROMEA_REPORT_COMMIT" == "$(git rev-parse --short HEAD)" ]]
export SOURCE_DATE_EPOCH="$(git show -s --format=%ct HEAD)"
export TZ=UTC GOAMD64=v1 GOARM64=v8.0
goreleaser check
if [[ "$mode" == snapshot ]]; then
  goreleaser release --snapshot --clean --skip=publish,sign
else
  goreleaser release --clean --skip=publish,sign
fi
git archive --format=tar --prefix="stromea-steuerbox-simulator-${release_version}/" HEAD | gzip -n > "dist/stromea-steuerbox-simulator_${release_version}_source.tar.gz"
python3 scripts/release-assets.py checksums dist "$release_version"
python3 scripts/release-assets.py verify dist "$release_version"
# Smoke-test only --version (no identity, listener or device access).
case "$(uname -m)" in arm64) host_arch=arm64 ;; x86_64) host_arch=amd64 ;; *) exit 1 ;; esac
smoke_tmp="$(mktemp -d)"
trap 'rm -rf "$smoke_tmp"' EXIT
tar -xzf "dist/stromea-steuerbox-simulator_${release_version}_darwin_${host_arch}.tar.gz" -C "$smoke_tmp" stromea-steuerbox-simulator
# Enforce the documented deployment target despite moving runner SDKs.
otool -l "$smoke_tmp/stromea-steuerbox-simulator" | grep -Eq 'minos +13\.0$'
version_output="$("$smoke_tmp/stromea-steuerbox-simulator" --version)"
echo "$version_output"
[[ "$version_output" == "Stromea Steuerbox-Simulator Version ${release_version} (Softwarestand ${STROMEA_REPORT_COMMIT})" ]]
(cd dist && shasum -a 256 -c SHA256SUMS)
