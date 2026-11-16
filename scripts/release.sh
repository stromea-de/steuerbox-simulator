#!/usr/bin/env bash
# Local only: publishing and GoReleaser's signing phase are always skipped.
set -euo pipefail
cd "$(dirname "$0")/.."
mode="${1:-snapshot}"
unsigned="${2:-}"
if [[ "$mode" != snapshot && "$mode" != tag ]] || [[ -n "$unsigned" && "$unsigned" != --unsigned ]] || [[ $# -gt 2 ]]; then
  echo 'Aufruf: scripts/release.sh [snapshot|tag] [--unsigned]' >&2
  exit 2
fi
if [[ "$(uname -s)" != Darwin ]]; then echo 'Release mit macOS-Menü benötigt macOS + Xcode Command Line Tools.' >&2; exit 1; fi
command -v goreleaser >/dev/null
command -v python3 >/dev/null
if [[ "$(goreleaser --version)" != *'2.20.0'* ]]; then
  echo 'GoReleaser OSS 2.20.0 erforderlich (docs/VERTEILUNG.md).' >&2
  exit 1
fi
if [[ "$unsigned" != --unsigned ]]; then
  command -v minisign >/dev/null
  # Without MINISIGN_SECRET_KEY the key comes from the macOS login keychain
  # (unlock first: security unlock-keychain). It only exists in a 0600 temp file during the build.
  if [[ -z "${MINISIGN_SECRET_KEY:-}" ]]; then
    keychain_service="${MINISIGN_KEYCHAIN_SERVICE:-stromea-release-minisign}"
    key_dir="$(umask 077; mktemp -d "${TMPDIR:-/tmp}/stromea-release-key.XXXXXX")"
    trap 'rm -P "$key_dir/release.key" 2>/dev/null || true; rmdir "$key_dir" 2>/dev/null || true' EXIT
    (umask 077; security find-generic-password -a stromea-release -s "$keychain_service" -w | base64 -d > "$key_dir/release.key")
    MINISIGN_SECRET_KEY="$key_dir/release.key"
  fi
  MINISIGN_PUBLIC_KEY="${MINISIGN_PUBLIC_KEY:-$HOME/.config/stromea-release/release.pub}"
  MINISIGN_SECRET_KEY="$(python3 scripts/distribution.py key "$MINISIGN_SECRET_KEY" "$PWD")"
  test -f "$MINISIGN_PUBLIC_KEY"
fi
# Report labels follow the checked-out tag; snapshot archive names are independent.
export STROMEA_REPORT_VERSION="$(git describe --tags --exact-match HEAD 2>/dev/null || true)"
export STROMEA_REPORT_COMMIT="$(python3 scripts/build-id.py)"
export SOURCE_DATE_EPOCH="$(git show -s --format=%ct HEAD)"
export TZ=UTC
goreleaser check
if [[ "$mode" == snapshot ]]; then
  goreleaser release --snapshot --clean --skip=publish,sign
  release_version=1.0.0
else
  if [[ -n "$(git status --porcelain)" ]]; then echo 'Tag-Build verlangt sauberen Checkout.' >&2; exit 1; fi
  release_tag="$(git describe --tags --exact-match HEAD)"
  release_version="${release_tag#v}"
  goreleaser release --clean --skip=publish,sign
fi
# Archive HEAD directly: GoReleaser source archives require a pre-existing tag.
# pipefail prevents a failed git archive from being signed as valid source.
mkdir -p dist
git archive --format=tar --prefix="stromea-steuerbox-simulator-${release_version}/" HEAD | gzip -n > "dist/stromea-steuerbox-simulator_${release_version}_source.tar.gz"
python3 scripts/distribution.py metadata dist "$release_version" https://github.com/stromea-de/steuerbox-simulator
python3 scripts/distribution.py checksums dist
if [[ "$unsigned" != --unsigned ]]; then
  minisign -S -s "$MINISIGN_SECRET_KEY" -m dist/SHA256SUMS -x dist/SHA256SUMS.minisig -t 'Stromea Steuerbox-Simulator release checksums'
  minisign -V -p "$MINISIGN_PUBLIC_KEY" -m dist/SHA256SUMS -x dist/SHA256SUMS.minisig
  cp "$MINISIGN_PUBLIC_KEY" dist/release.pub
else
  echo 'Explizit unsignierter Entwicklungsbuild; nicht als signiertes Release verteilen.' >&2
fi
echo 'Lokale Artefakte in dist/. Kein Upload. Snapshots sind keine Veröffentlichungsfreigabe.'
