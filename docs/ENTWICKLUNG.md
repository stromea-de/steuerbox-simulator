# Entwicklung

Stand: 08.11.2026. Go-Modulvorgabe: Go 1.26.0+. Die lokale Paketierung wurde mit Go 1.27.3 und GoReleaser OSS 2.20.0 geprüft. Die Abhängigkeiten sind in `go.mod` und `go.sum` fixiert; Lizenzprüfung siehe [LIZENZEN.md](LIZENZEN.md).

## Aufbau

- `cmd/steuerbox-simulator`: CLI, Benutzerzustand und lokaler HTTP-Server.
- `internal/identity`: Identität, Zertifikate und Vertrauensdaten.
- `internal/discovery`: DNS-SD und ausdrücklich gestartete Gerätesuche.
- `internal/eebus`: enbility-SHIP/SPINE-Anbindung und lokales Testgerät.
- `internal/inspection`, `internal/session`: Prüfablauf, quittungsbasierte Bewertung und Fortschritt.
- `internal/report`: eigener PDF-Writer mit Standardfonts, ohne PDF-/Fontbibliothek.
- `internal/webui`: eingebettete mehrsprachige Weboberfläche und lokale HTTP-API.

Der Modulpfad lautet `github.com/stromea-de/steuerbox-simulator`, der CLI-Quellpfad `cmd/steuerbox-simulator`. Programm- und Paketname sind `stromea-steuerbox-simulator`. Benutzerzustände liegen unter diesem Namen. `--state` wählt ausdrücklich einen anderen Pfad.

## Lokale Prüfungen

```sh
go test ./...
./scripts/check.sh
```

`check.sh` verlangt Go, Bash, Python 3.11+, Node.js 20+ sowie die unten genannten Browser-/PDF-Prüfwerkzeuge und prüft Formatierung, Modulprüfsummen, statische Analyse, Race Detector, Paketierungsregressionen und sechs Cross-Builds. Der Race Detector benötigt eine passende C-Toolchain. Die Tests verwenden Simulatoren und Loopback, einschließlich separater Testprozesse mit echter SHIP/TLS-Verbindung. Die Umgebung muss TCP-/UDP-Listener auf Loopback erlauben; verweigerte Listener sind ein fehlgeschlagener Testlauf. Reale Geräte und LAN werden dafür nicht benötigt.

Die vollständige Prüfung benötigt einen macOS-Buildhost mit Xcode Command Line Tools/Swift (auch für die beiden macOS-Cross-Builds und die PDFKit-Prüfung). Playwright-Core 1.64.1 (Apache-2.0), ein dazu passendes Chromium und Google Chrome müssen bereits lokal vorhanden sein. Es werden keine Browser oder npm-Pakete automatisch installiert. Standardpfad für den Playwright-Import: `$HOME/.cache/stromea-pw/node_modules/playwright-core/index.mjs`; mit `E2E_PLAYWRIGHT` kann ein anderer vorhandener Importpfad angegeben werden. Fehlende Werkzeuge oder Browser lassen die Prüfung fehlschlagen.

`check.sh` führt jede verbindliche UX-Entscheidung mit `test_ux_entscheidungen.mjs` aus und prüft die Übereinstimmung mit dem mitgelieferten Katalog `scripts/fixtures/ux036.json`. Die vollständigen bestehenden E2E-Abläufe für Chrome und Chromium sowie beide Prüfrichtungen laufen ebenfalls. Dafür wird ein aktuelles lokales Binary gebaut. Browserprofile und Serverzustände sind isoliert; sämtliche Verbindungen führen zu Loopback-Testpartnern oder ausdrücklich gesetzten Mocks. Rohlogs, DOM-Auszüge und Screenshots liegen unter `artifacts/036/`. Der private Designprototyp wird nicht exportiert; der gemeinsame Produktablauf wird im Export vollständig geprüft.

Zusätzliche WebKit-Regressionsprüfung mit dem vorhandenen Systemrenderer:

```sh
STROMEA_009_UI_ARTIFACTS=1 go test ./internal/webui -run TestWizard
swift -module-cache-path /tmp/stromea-swift-cache scripts/test_webui.swift --screenshots
swift -module-cache-path /tmp/stromea-swift-cache scripts/test_webui.swift --screenshots --dark
```

Diese zusätzliche WebKit-Browserregression verwendet synthetische API-Antworten und weist keine Geräteverbindung nach. Die verpflichtenden Playwright-Abläufe prüfen dagegen echte Verbindungen mit dem Loopback-Testgerät. Optionale zusätzliche PDF-Textprüfungen verwenden vorhandenes `pdftotext`; fehlt das Programm, wird nur diese Zusatzprüfung übersprungen. Die PDFKit-Prüfung in `check.sh` bleibt verpflichtend. Keine Tests greifen auf reale Geräte zu. Erzeugte Darstellungen gehören nach `artifacts/`, Builds nach `bin/` oder `dist/`; diese Verzeichnisse werden nicht eingecheckt.

## Offline-QR-Bibliothek

`@paulmillr/qr` 0.3.0 ist unter MIT eingebettet. Das Paket benötigt keine Abhängigkeiten. `internal/webui/assets/qr-vendor.json` hält Quelladresse und SHA-512 fest. Nach Herunterladen des dort bezeichneten npm-Archivs reproduziert `python3 scripts/bundle-qr.py /pfad/qr-0.3.0.tgz` den lokalen Bundle. `node scripts/test_inputs.js` prüft Dezimalwerte, SHIP-Daten und einen vollständigen Encoder-/Decoder-Durchlauf ohne Netzwerk.

## Version und Build

Ohne injizierte Version bleibt `internal/report.Version` leer (Testversion). Der CI-Wrapper setzt am Tag `v1.0.0` über `-X` die Version `1.0.0` und den sauberen Commit; auch die optionale lokale Homebrew-Formula setzt die Paketversion. Manueller Build:

```sh
go build -trimpath -buildvcs=false \
  -ldflags='-s -w -buildid= -X github.com/stromea-de/steuerbox-simulator/internal/report.Version=1.0.0' \
  -o bin/stromea-steuerbox-simulator ./cmd/steuerbox-simulator
./bin/stromea-steuerbox-simulator serve
```

Mit `serve` die angezeigte URL selbst im Browser öffnen. Ohne Unterbefehl öffnen macOS und Windows den Browser und das Desktopmenü automatisch. Linux bleibt im Terminalbetrieb. Der HTTP-Standardport ist `127.0.0.1:33287`. Bei belegtem Port ausdrücklich `serve --listen 127.0.0.1:33288` wählen.

Entwicklungsregeln: [CONTRIBUTING.md](../CONTRIBUTING.md), Betriebsgrenzen: [SICHERHEIT.md](SICHERHEIT.md), lokale Pakete: [VERTEILUNG.md](VERTEILUNG.md).


Die ergänzte Flow-Regressionssuite läuft lokal mit `swift -module-cache-path /tmp/stromea-webui-cache scripts/test_webui.swift --flow015 --screenshots`; `--dark` prüft den Dunkelmodus. Die Antworten bleiben synthetisch, QR-Bilder werden im Browser erzeugt. Die Suite prüft manuelle Kopplung nach leerer Suche, QR-Import, Tooltips, Prüfumfang, Diktatprüfung und vorzeichenrichtige Leistungsrichtungen einschließlich Messlücken. Im Diagramm bedeutet ein positiver Wert Bezug, ein negativer Einspeisung.

Leistungsfelder akzeptieren Dezimalkomma oder Dezimalpunkt mit bis zu drei Nachkommastellen. Tausenderzeichen weglassen: `4200`. Mehrdeutige Eingaben wie `4.200` werden abgelehnt; `4,200` ist ausdrücklich ein Dezimalwert. Die CLI verwendet dieselbe Regel.

Browserdiktat ist optional und wird nur bei vorhandener Sprach-API angeboten. Vor der ersten Aufnahme erklärt die Anwendung mögliche Audioübertragung durch den Browser und verlangt eine Bestätigung pro Sitzung. Die Anwendung verarbeitet oder speichert kein Audio. QR-Fotos und QR-Texte werden weiterhin lokal dekodiert. Der gemeinsame Parser übernimmt diktierte Viererblöcke erst nach Prüfung; unbekannte Wörter bleiben sichtbar.

`/api/state` liefert den gewählten Prüfumfang, gemeldete Gerätefähigkeiten und bis zu 600 Leistungsproben (`at`, `watts`, `step`). `watts: null` kennzeichnet eine Leselücke. Der Zeitstempel beschreibt den lokalen Lesezeitpunkt. Der Puffer umfasst die jüngsten Proben; bei langen Wartezeiten fallen ältere Messpunkte heraus. JSON und PDF enthalten dieselben aufgezeichneten Proben. Der explizite Bezugsumfang wertet keine Einspeisefunktion als fehlenden Pflichtnachweis. Der nicht gewählte Langtest bleibt im Bericht offen; alte Berichte ohne expliziten Prüfumfang behalten ihre bisherige Bewertung.

### Lokale Sprachen und Gerätefunktionen

Die UI lädt sieben eingebettete Wörterbücher aus `internal/webui/assets/i18n/`.
Nach einer Änderung erzeugt `python3 scripts/build-i18n.py` das Offline-Bundle neu.
`python3 scripts/test_i18n016.py` prüft Schlüssel, Werte, statische Texte und Bundle;
`node scripts/test_i18n016.js` prüft Browserpräferenz und Sprach-Rückfallebene.
`--flow016` im vorhandenen WebKit-Harness prüft Kopplungszustände, Sprachen und Layout.
Maschinelle Übersetzungen sind in der Oberfläche als ungeprüft markiert. PDF bleibt Deutsch.

Die Zustandsantwort unterscheidet TLS-Geräteidentität (`trust.device`) und bestätigtes
SHIP-Vertrauen (`trust.simulator`). Lokales Bestätigen setzt diese Belege nicht.
Der gemeldete Gerätetyp wird unverändert gespeichert; `HeatGenerationSystem` ist ein
Wärmeerzeuger, kein Beleg für eine Wärmepumpe. Der gewählte Umfang ist `consumption`,
`production` oder `both`; die Prüfung schreibt nur die ausgewählten unterstützten Richtungen.
Quelle: lokal geprüfte APIs von spine-go v0.7.0 und eebus-go v0.7.0, 29.10.2026.
