#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
command -v node >/dev/null || { echo 'Node.js und Playwright mit Chrome/Chromium sind für die Browserprüfungen erforderlich.' >&2; exit 1; }
mkdir -p artifacts/036
if [[ -z "${E2E_BINARY:-}" ]]; then
  build_commit="$(python3 scripts/build-id.py)"
  go build -trimpath -ldflags "-X github.com/stromea-de/steuerbox-simulator/internal/report.Commit=$build_commit" -o artifacts/036/check-server ./cmd/steuerbox-simulator
  export E2E_BINARY=artifacts/036/check-server
fi
REVIEW036D_OUTPUT=artifacts/036d node scripts/test_review036d.mjs
node scripts/test_pairing036d.mjs
node scripts/test_release045.mjs
mkdir -p artifacts/036e/release-pdf
RELEASE036E_OUTPUT="$PWD/artifacts/036e/release-pdf" go test ./internal/report -run TestReleaseSentence036e -count=1
swift -module-cache-path artifacts/036/swift-cache scripts/test_release036e.swift artifacts/036e/release-pdf
UX036_PROBES_ONLY=0 UX036_WIDTHS=1920,1440,1280,820,390 UX036_OUTPUT=artifacts/036 node scripts/test_ux_entscheidungen.mjs
node scripts/test_copy036c.mjs
QA039_OUTPUT=artifacts/039 node scripts/test_qa039.mjs
node scripts/test_ux_s18_race039.mjs
# 043: Nutzerweg Echtgerät gegen den Loopback-Testpartner (Verbinden → Profil → zurück → Abbruch → Neu verbinden → Prüfer-Dialog → läuft).
UX043_OUTPUT=artifacts/043 node scripts/test_ux043.mjs
for browser in chrome chromium; do
  node scripts/e2e_034.mjs --browser="$browser" --output=artifacts/036/e2e
  node scripts/e2e_034.mjs --browser="$browser" --scopes-only=1 --output=artifacts/036/e2e
done
swift -module-cache-path artifacts/036/swift-cache scripts/test_pdf036.swift artifacts/036/screens/pdf \
  artifacts/034-e2e/chrome/hell-1440-onsite.pdf \
  artifacts/034-e2e/chrome/hell-1440-revision.pdf \
  artifacts/034-e2e/chrome/hell-1440-scope-both.pdf \
  artifacts/034-e2e/chrome/hell-1440-scope-production.pdf

if [[ -f scripts/build-prototyp.py ]]; then
  node scripts/test_prototype036e.mjs
  swift -module-cache-path artifacts/036/swift-cache scripts/test_pdf036.swift artifacts/036/screens/prototype-pdf artifacts/prototyp-*.pdf
fi
swift -module-cache-path artifacts/036/swift-cache scripts/test_pdf036.swift --copy-only artifacts/036/pdf-copy artifacts/034-e2e/chrome/*.pdf artifacts/034-e2e/chromium/*.pdf
