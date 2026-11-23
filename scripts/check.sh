#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export E2E_OUTPUT="${E2E_OUTPUT:-artifacts/036}"
if [[ -n "$(gofmt -l cmd internal)" ]]; then
  echo 'Go-Dateien sind nicht formatiert.' >&2
  exit 1
fi
go mod verify
go vet ./...
go test -race -count=1 ./...
command -v node >/dev/null || { echo 'Node.js ist für die Browser- und Regressionstests erforderlich.' >&2; exit 1; }
node scripts/test_result024c.js; node scripts/test_result040.js; node scripts/test_title042.js; node scripts/test_text034.js; node scripts/test_copy035.js; node scripts/test_review034.js; node scripts/test_inputs.js; node scripts/test_network033.js; node scripts/test_guide020.js; node scripts/test_i18n016.js; node scripts/test_restore047.js
python3 scripts/test_i18n016.py
python3 scripts/test_i18n034.py
python3 scripts/test_i18n040.py
python3 scripts/test_distribution.py
python3 scripts/test_version024c.py
python3 scripts/test_build025c.py
if [[ -f scripts/test_export_public.py ]]; then
  python3 scripts/test_export_public.py
  python3 scripts/test_prototyp.py
  bash -n scripts/export-public.sh
fi
bash -n scripts/check.sh scripts/check-browser036.sh scripts/release.sh scripts/appimage.sh
scripts/check-browser036.sh
for target in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
  scripts/build-desktop.sh "${target%/*}" "${target#*/}"
  echo "Build erfolgreich: $target"
done
