# Pakete, Release-Build und Verifikation

Stand: 24.11.2026. Aktuelle Version: **1.0.3** (Tag `v1.0.3`); erste öffentliche Version: **1.0.0**, Tag `v1.0.0`,
AGPL-3.0-only. Projekt: https://github.com/stromea-de/steuerbox-simulator.

## Öffentlicher Releasepfad

Das öffentliche Repository beginnt mit frischer Historie aus dem geprüften
Allowlist-Export, einem Commit „Stromea Steuerbox-Simulator 1.0.0“ und dem Tag
`v1.0.0`. Jede Folgeversion ist ein weiterer Commit „Stromea Steuerbox-Simulator X.Y.Z“
aus einem neuen geprüften Export mit Tag `vX.Y.Z`. Die private Entwicklungshistorie wird
nicht übernommen.

`.github/workflows/release.yml` baut bei Tags `v*.*.*` auf `macos-latest` mit
Go **1.27.3** und GoReleaser OSS **2.20.0**. Der Installer prüft eine fest
hinterlegte Upstream-SHA256 und die Toolversion. Cocoa benötigt macOS und Xcode
Command Line Tools, CGO bleibt unter Windows/Linux aus. Das macOS-Deployment-
Target ist explizit 13.0 und wird am erzeugten Binary geprüft. Der Wrapper
`scripts/release-ci.sh tag` verlangt einen sauberen Checkout und einen exakten
stabilen Tag. `--version` nennt die Tagversion (z. B. `1.0.3`) und den Commit ohne Test-/Dirty-Zusatz.

Nach Go-Tests und Build prüft der Workflow Dateiliste und Prüfsummen, erzeugt
GitHub Artifact Attestations für alle zwölf Dateien und verifiziert sie. Erst
anschließend erstellt er ein reguläres Release mit den englisch/deutschen
[Release Notes](RELEASE_NOTES_1.0.3.md). Standard ist direkte Veröffentlichung.
Die Repositoryvariable `RELEASE_DRAFT=true` erstellt stattdessen einen Entwurf;
dessen Veröffentlichung erfolgt danach manuell. Es gibt keine Vorabversion.
Attestations und Veröffentlichung sind auf das öffentliche Zielrepository
beschränkt. `workflow_dispatch` mit `mode=build-only` baut auch in einem privaten
Staging-Repository, ohne Attestations und Release. Die Ergebnisse liegen sieben
Tage als Actions-Artefakt vor. Der Workflow muss dort an seinem exportierten
Pfad `.github/workflows/release.yml` auf dem Standardbranch liegen.

## Dateien

Präfix aller Archive/Pakete: `stromea-steuerbox-simulator_1.0.3_`.

| Suffix | Plattform |
| --- | --- |
| `darwin_amd64.tar.gz`, `darwin_arm64.tar.gz` | macOS Intel / Apple Silicon |
| `linux_amd64.tar.gz`, `linux_arm64.tar.gz` | Linux x86_64 / ARM64 |
| `windows_amd64.zip`, `windows_arm64.zip` | Windows x64 / ARM64 |
| `linux_amd64.deb`, `linux_arm64.deb` | Debian-Pakete |
| `linux_amd64.rpm`, `linux_arm64.rpm` | RPM-Pakete |
| `source.tar.gz` | zugehöriger Quellcode des getaggten Commits |

Dazu kommt `SHA256SUMS`, ausschließlich mit diesen elf Downloads. Keine
Homebrew-/Scoop-/winget-Metadaten, kein AppImage, kein minisign-Schlüssel und
keine minisign-Signatur in den öffentlichen Releases ab 1.0.0.

## Prüfen

GitHub CLI mit Attestation-Unterstützung separat bereitstellen. Vor Auspacken,
Start oder Installation Prüfsummenliste und gewählten Download verifizieren:

```sh
gh attestation verify SHA256SUMS --repo stromea-de/steuerbox-simulator
gh attestation verify stromea-steuerbox-simulator_1.0.3_darwin_arm64.tar.gz --repo stromea-de/steuerbox-simulator
# Alle elf Downloads müssen vorliegen:
shasum -a 256 -c SHA256SUMS
# Bei nur einem Download genau dessen Eintrag prüfen:
grep '  stromea-steuerbox-simulator_1.0.3_darwin_arm64.tar.gz$' SHA256SUMS | shasum -a 256 -c -
```

Linux kann `sha256sum` verwenden. Unter Windows zunächst ebenfalls beide
Attestations mit `gh attestation verify` prüfen, dann
`Get-FileHash .\stromea-steuerbox-simulator_1.0.3_windows_amd64.zip -Algorithm SHA256`
mit genau dem Eintrag in der verifizierten Liste vergleichen. Bei Fehler anhalten.

## Systemvoraussetzungen

64-Bit-Betriebssystem und passendes Archiv: macOS **13 Ventura oder neuer**,
Windows **10 oder neuer** bzw. Windows Server **2016 oder neuer**, Linux mit
passender amd64-/arm64-Architektur (Go-Basis: Kernel 3.2 oder neuer).
Go 1.27 bestimmt diese Untergrenzen; vollständige Laufzeittests auf allen
Mindestversionen stehen aus. Linux-Binaries verwenden kein CGO und benötigen
keine nachinstallierte Go-Toolchain. Ein aktueller Browser und ein beschreibbarer
Benutzer-Konfigurationsordner sind erforderlich. Für fertige Binaries sind
Go, Python, Xcode und GoReleaser nicht nötig; GitHub CLI wird nur zur
Provenienzprüfung benötigt.
Quelle: [Go-Mindestanforderungen](https://go.dev/wiki/MinimumRequirements),
geprüft 16.11.2026.

## Start und Betriebssystemwarnungen

macOS hat **keine Developer-ID-Signatur und keine Apple-Notarisierung**.
Gatekeeper kann Downloads wegen Quarantäne bzw. eines nicht verifizierten
Entwicklers blockieren. Nach erfolgreicher Herkunfts-/Hashprüfung im Finder
Rechtsklick → Öffnen verwenden, ggf. unter Datenschutz & Sicherheit zulassen.
Alternativ am entpackten Binary:
`xattr -d com.apple.quarantine ./stromea-steuerbox-simulator`.
Das entfernt Quarantäne und fügt keine Notarisierung hinzu.

Windows hat **keine Authenticode-Signatur**; SmartScreen kann warnen oder
blockieren. Eine Hash-/Attestation-Prüfung beseitigt diese Warnung nicht.

Nach dem Entpacken macOS: `./stromea-steuerbox-simulator` (Menüleistenbetrieb),
Windows: Doppelklick auf `stromea-steuerbox-simulator.exe` (Infobereich),
Linux: `./stromea-steuerbox-simulator serve`. Expliziter Terminalbetrieb auf
macOS: `./stromea-steuerbox-simulator serve`. Das Windows-Release ist ein
GUI-Build (`-H=windowsgui`); vorgesehen ist der Doppelklick.
`.\stromea-steuerbox-simulator.exe serve` kann den Server starten, garantiert
aber keine sichtbare Konsolenausgabe und kein Terminal-Vordergrund-/Strg+C-
Verhalten. Dies betrifft auch Versions- und Fehlerausgaben. Für Terminalbetrieb
in PowerShell aus dem ausgecheckten Quellcode **ohne `-H=windowsgui`** bauen:

```powershell
$env:CGO_ENABLED = '0'
go build -trimpath -buildvcs=false -o stromea-steuerbox-simulator-console.exe ./cmd/steuerbox-simulator
.\stromea-steuerbox-simulator-console.exe serve
```

Go ist nur für diesen Quellcodebuild erforderlich. Eine native Windows-Abnahme
von `serve`, `--version`, Fehlerausgabe und Strg+C steht aus.
Browser: `http://127.0.0.1:33287`.
Keine eingehende LAN-Freigabe für die lokale Weboberfläche erforderlich.
Für Gerätebetrieb gelten die expliziten Freigaben in der Anwendung.

Linux-Pakete nach Verifikation: `sudo apt install ./DATEI.deb` oder
`sudo dnf install ./DATEI.rpm`, Start danach `stromea-steuerbox-simulator serve`.
Zielsysteminstallation von DEB/RPM und Windows-Ausführung sind gesondert zu prüfen.

Daten liegen im Benutzer-Konfigurationsordner unter
`stromea-steuerbox-simulator/`, Berichte in `reports/`, Desktop-Logs in
`logs/simulator.log` (ab 5 MiB beim nächsten Start eine Vorgängerdatei).
Portable Dateien löschen deinstalliert das Programm; Daten bleiben separat.

## Lokale Probe und Reproduzierbarkeit

```sh
bash scripts/install-goreleaser.sh /pfad/zu/lokalen-tools
# Toolverzeichnis in PATH aufnehmen; Go 1.27.3, Git, Bash, Python 3.11+, macOS + Xcode
bash scripts/release-ci.sh snapshot
# Sauberer Checkout am vorhandenen Tag:
bash scripts/release-ci.sh tag
```

Beide Befehle bauen ausschließlich lokal. Snapshotname und Binaryversion sind
`1.0.0`, aber nur der Tagpfad ist ein Releasekandidat. `scripts/release.sh`
bleibt eine optionale lokale minisign-Probe (auch `snapshot --unsigned` möglich);
diese Signaturen werden nicht öffentlich verwendet. Lokal erzeugte
Paketmanagerentwürfe gehören nicht zu den CI-Release-Assets.

Gleicher sauberer Commit, Go-/GoReleaser-Version, Modulprüfsummen, Host-SDK und
Architektur sind Voraussetzungen für Bytevergleiche. `-trimpath`,
`-buildvcs=false`, leere Build-ID, GOAMD64=v1, GOARM64=v8.0 sowie Commit-Zeitstempel
und `SOURCE_DATE_EPOCH` begrenzen veränderliche Eingaben. `macos-latest` und sein
SDK sind beweglich; keine uneingeschränkte Reproduzierbarkeitszusage über Runner-
Updates. Das Quellarchiv entsteht aus `git archive HEAD | gzip -n`.

Bei Weitergabe vollständige Projektlizenz, zugehörigen Quellcode und
Fremdhinweise bereitstellen. Alle Archive und Pakete enthalten die Lizenztexte.

Die EEBUS-Wort-/Bildmarke ist eine eingetragene Marke der EEBus Initiative e.V. EEBUS bezeichnet hier den unterstützten Kommunikationsstandard. Der Stromea Steuerbox-Simulator ist ein unabhängiges Open-Source-Projekt und wird weder von der EEBus Initiative e.V. noch von der EEBUService GmbH herausgegeben oder zertifiziert. Er ist keine Steuerbox-Hardware und ersetzt keine Konformitätsprüfung.

Der vorbereitete Ablauf steht in [scripts/publish-checklist.md](../scripts/publish-checklist.md).
