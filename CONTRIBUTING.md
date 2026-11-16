# Mitmachen beim Stromea Steuerbox-Simulator

Wir freuen uns über jeden Beitrag! Unser Ziel ist ein verlässliches, unabhängiges und einfach bedienbares Prüfwerkzeug für das gesamte Elektrohandwerk und Fachanwender.

---

## Wie du helfen kannst

1. **Geräte vor Ort testen & Kompatibilität melden:**  
   Das Wertvollste für die Community sind bestätigte Tests mit echten Wärmepumpen, Wallboxen, Speichern und Wechselrichtern. Nutze dafür unsere [Gerätekompatibilitäts-Vorlage](https://github.com/stromea-de/steuerbox-simulator/issues/new?template=geraetekomp_melden.md).

2. **Fehler melden & Feedback geben:**  
   Funktioniert ein Schritt nicht wie erwartet oder ist ein Text unklar? [Erstelle ein Issue](https://github.com/stromea-de/steuerbox-simulator/issues/new?template=bug_report.md) mit kurzer Fehlerbeschreibung.

3. **Code & Verbesserungen beitragen:**  
   Ob Bugfix, verbesserte Benutzerführung oder neue Schnittstellen – Pull Requests sind herzlich willkommen.

---

## Entwicklung & Pull Requests

### Voraussetzungen
- **Go 1.26.0 oder neuer**
- Git, Bash und Python 3.11+ für die vollständigen lokalen Prüfungen
- C-Toolchain für den Go Race Detector
- Node.js 20+, vorhandenes Playwright-Core 1.64.1 (Apache-2.0), dazu passendes Chromium und Google Chrome für die verpflichtenden Browserregressionen
- macOS mit Xcode Command Line Tools/Swift für die vollständige Prüfung einschließlich PDFKit und macOS-Builds; Details unter [Entwicklung](docs/ENTWICKLUNG.md)

### Lokaler Workflow
1. Repository forken und Branch erstellen:
   ```sh
   git checkout -b feature/mein-beitrag
   ```
2. Vor jedem Commit die automatischen Prüfungen ausführen:
   ```sh
   ./scripts/check.sh
   ```
   *`check.sh` prüft Formatierung (`gofmt`), statische Analyse (`go vet`), Race-Conditions und führt Cross-Builds durch.*
3. Pull Request mit einer kurzen, klaren Beschreibung einreichen.

---

## Grundsätze & Code-Qualität

- **Verständlich vor Kompliziert:** Halte Oberflächentexte und Fehlermeldungen in klarer Alltagssprache. Technische Details gehören in Tooltips oder einklappbare Bereiche.
- **Lokale Kernprüfung & Datensparsamkeit:** Kernprüfung und Berichte funktionieren ohne Cloud-Dienst oder Benutzerkonto; die Anwendung enthält keine eigene Telemetrie und startet keine ungefragten Netzwerksuchen. Optionales Browserdiktat kann nach deiner Zustimmung Audio an den Spracherkennungsdienst des Browsers senden und eine Internetverbindung benötigen. Siehe [Sicherheitsgrenzen](docs/SICHERHEIT.md).
- **AGPL-3.0-Konformität:** Alle Beiträge werden unter der AGPL-3.0-only lizenziert. Neue Fremdabhängigkeiten müssen permissiv lizenziert sein und vor Verwendung mit Version, Lizenz und vollständigen Hinweisen in `docs/LIZENZEN.md` erfasst werden.
- **Tests zuerst:** Änderungen in kleinen prüfbaren Schritten entwickeln; Tests ausschließlich gegen Simulatoren und Loopback. Reale Geräte oder LAN nur mit ausdrücklicher Freigabe.
- **Nachweisgrenzen:** Kein Anspruch auf Zertifizierung; Berichte ersetzen keine Konformitätsprüfung. Details: [Sicherheitsgrenzen](docs/SICHERHEIT.md).
