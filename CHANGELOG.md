# Änderungsprotokoll (Changelog)

Alle wichtigen Änderungen am Stromea Steuerbox-Simulator werden in diesem Dokument festgehalten.

Das Format basiert auf [Keep a Changelog](https://keepachangelog.com/de/1.0.0/) und dieses Projekt hält sich an [Semantic Versioning](https://semver.org/lang/de/).

---

## [1.0.2] - 2026-11-23

### Geändert
- **Originalwerte des Geräts technisch gesichert:** Vor jeder Vor-Ort-Prüfung liest der Simulator Notbetriebswerte (Leistung, Mindestdauer) und Grenzen frisch vom Gerät und speichert sie je Gerät, bevor er etwas schreibt. Am Ende werden sie zurückgeschrieben und zurückgelesen; keine Grenze bleibt aktiv. Bricht der Simulator ab oder wird er hart beendet, stellt er beim nächsten Start die Originalwerte automatisch wieder her. Schreibt nur, was abweicht; ein unverändertes Gerät bleibt unberührt.
- Ein Gerät kann nur von einem Simulatorfenster gleichzeitig geprüft werden.
- Eine Statuszeile zeigt, ob die Originalwerte wiederhergestellt sind.

### Behoben
- Geräte, die im Notbetrieb neue Notbetriebswerte ablehnen oder LPC und LPP eine gemeinsame Mindestdauer führen, werden zuverlässig wiederhergestellt.

## [1.0.1] - 2026-11-19

### Behoben
- **Notbetrieb (Failsafe) erst nach Ablauf der Ausfallfrist bewertet (LPC und LPP):** 1.0.0 konnte „Failsafe gemessene Wirkung“ unmittelbar nach dem Heartbeat-Stopp als bestanden werten, wenn die Leistung unter der aktiven Grenze schon unter dem Notbetriebswert lag. Jetzt gilt bei Geräten mindestens die Protokollfrist von 120 s (LPC/LPP 1.0.0) ab Stopp bzw. späterer Quittung plus 30 s Reserve. Bestanden nur bei einem Leistungsabfall, der erst durch den Notbetrieb erklärbar ist, oder wenn das Gerät den Notbetrieb meldet; sonst „nicht geprüft“. Rückkehrschritte ohne belegten Notbetrieb bleiben „nicht geprüft“; unbelegter Notbetrieb erscheint im Messverlauf hell gepunktet. Wiederholte Messwerte mit altem Gerätezeitstempel zählen nicht. Übungsmodus unverändert.
- Mit 1.0.0 vor Ort erstellte Notbetriebsnachweise bitte mit 1.0.1 wiederholen.

## [1.0.0] - 2026-11-17

Erste stabile Version des Stromea Steuerbox-Simulators als eigenständiges Prüfwerkzeug für Fachbetriebe und Installateure.

Kein Zertifizierungsanspruch; Berichte ersetzen keine Konformitätsprüfung.

### Seit der ersten Vorbereitung ergänzt
- Sieben lokale UI-Sprachen; Übersetzungen sind KI-geprüft, ohne Muttersprachler-Lektorat; PDF weiterhin Deutsch.
- Gespeicherte Berichte, CSV-Export und überarbeitete Kopplungs-/Prüfansichten.
- Präzisere Quittungs-, Failsafe- und Freigabenachweise; fehlende Messungen bleiben offen.
- Stabile Auswahl des Prüfumfangs über Statusabfragen hinweg; korrigierte Netzwerkauswahl und Kopplungsnavigation.
- Go 1.26.0+ als Modulvorgabe; lokal geprüfte Release-Toolchain Go 1.27.3 und GoReleaser OSS 2.20.0.
- Releasevorbereitung mit GitHub Actions, zwölf festen Release-Assets und GitHub Artifact Attestations einschließlich SHA256SUMS.
- Gesicherte lokale Datenübertragung per Export/Import und überarbeitete Verbindungs-/Abbruchhinweise.
- Versionierte Downloads und lokale Prüfskripte im öffentlichen Quellbaum.
- CLI-Auskunft mit `--version`/`version` ohne Anlegen einer Identität.

### Hinzugefügt
- **Geführter 3-Schritt-Assistent:** Moderne, übersichtliche Weboberfläche im Browser (`1. Gerät koppeln` → `2. Prüfung einstellen` → `3. Prüfen & Bericht`).
- **Standardport 33287:** HTTP-Port der Weboberfläche entspricht „EEBUS“ auf der Telefontastatur (3-3-2-8-7).
- **Integrierter Übungsmodus:** Vollständige Simulation von Steuerbox und Testgerät auf dem lokalen Rechner ohne externe Hardware.
- **Gerät-vor-Ort-Modus:** Gezielte mDNS-Geräteerkennung im lokalen Subnetz, manueller Fallback und sichere SKI-Schlüsselbestätigung.
- **Acht Prüfbereiche mit einzelnen Prüfschritten:** Automatische Überprüfung von TLS/SHIP-Verbindung, EEBUS Use Cases (LPC nach §14a EnWG, LPP nach §9 EEG, MPC-Leistungsmessung), Failsafe-Werten, Heartbeat, Abregelung, Signalausfall, Signalwiederkehr und bestätigter Grenzfreigabe. Fehlende Nachweise und unbestätigte Freigabe werden ausdrücklich ausgewiesen.
- **Zweistufiger PDF-Prüfbericht:** Seite 1 als verständliche Kunden- und Betreiberübersicht; ab Seite 2 technischer Anhang mit Zeitstempeln, Quittungen und Messwerten.
- **JSON-Datenexport:** Strukturierter Rohdatenexport für Archiv- und ERP-Systeme.
- **Freiwillige Unterschriftsfelder:** Optionale Felder für Empfangsbestätigung und Prüfvermerk auf dem PDF-Bericht.
- **Multi-Plattform-Paketierung:** Buildkonfiguration für Einzeldateien für Windows (x64 & ARM64), macOS (Apple Silicon & Intel) und Linux (x86_64 & ARM64 inkl. `.deb` / `.rpm`).
- **Lokale Kernprüfung und Berichte:** Ohne Cloud-Dienst oder Benutzerkonto; keine eigene Telemetrie. Optionales Browserdiktat kann nach Zustimmung Audio an den Spracherkennungsdienst des Browsers senden und Internet benötigen.
