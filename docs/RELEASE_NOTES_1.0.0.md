# Stromea Steuerbox-Simulator 1.0.0

Release date / Veröffentlichungsdatum: 2026-11-17

## English

First stable release: local browser interface, guided EEBUS checks, a loopback
practice mode, PDF reports, saved reports and JSON/CSV export. Seven interface
languages; translations have been AI-reviewed, without native-speaker editing.
PDF reports remain in German. This is a software simulator, with no certification
claim; its reports do not replace a conformity assessment. Device communication
requires explicit authorization and confirmation of the device key (SKI).

Downloads: six portable archives (macOS/Linux/Windows, amd64/arm64), four Linux
packages (DEB/RPM, amd64/arm64), source archive and SHA256SUMS. No package-manager
metadata, minisign signature or signing key is shipped. GitHub Artifact
Attestations cover each of these twelve assets, including SHA256SUMS.

## Deutsch

Erste stabile Version: lokale Browseroberfläche, geführte EEBUS-Prüfung,
Übungsmodus auf Loopback, PDF-Berichte, gespeicherte Berichte und JSON-/CSV-Export.
Sieben UI-Sprachen; Übersetzungen sind KI-geprüft, ohne Muttersprachler-Lektorat.
PDF-Berichte bleiben deutsch. Software-Simulator ohne Zertifizierungsanspruch;
der Bericht ersetzt keine Konformitätsprüfung. Gerätekommunikation erfordert
Freigabe und Bestätigung des Geräteschlüssels (SKI).

## Verify / Prüfen

Download the desired asset and SHA256SUMS from the same release. Verify both
before extracting or installing (GitHub CLI with attestation support required):

```sh
gh attestation verify SHA256SUMS --repo stromea-de/steuerbox-simulator
gh attestation verify stromea-steuerbox-simulator_1.0.0_darwin_arm64.tar.gz --repo stromea-de/steuerbox-simulator
# If ALL eleven downloads are present / wenn ALLE elf Downloads vorliegen:
shasum -a 256 -c SHA256SUMS
# For just the selected download / bei nur einem Download:
grep '  stromea-steuerbox-simulator_1.0.0_darwin_arm64.tar.gz$' SHA256SUMS | shasum -a 256 -c -
```

For another platform, substitute its exact asset name; Linux can use `sha256sum`.
On Windows, verify provenance with `gh attestation verify` for SHA256SUMS and the
ZIP, then compare `Get-FileHash .\stromea-steuerbox-simulator_1.0.0_windows_amd64.zip -Algorithm SHA256`
with its entry in SHA256SUMS. Stop on any verification failure.

macOS: no Apple Developer ID signature or notarization. A quarantined download
may be blocked as an unverified developer. After successful verification, use
Finder → right-click → Open; if necessary allow it in Privacy & Security. An
alternative is `xattr -d com.apple.quarantine ./stromea-steuerbox-simulator` on
the extracted binary. This removes quarantine; it does not notarize the binary.
Windows: no Authenticode signature; SmartScreen may warn or block execution.

macOS ist nicht notarisiert und hat keine Developer-ID-Signatur. Gatekeeper kann
den Download wegen Quarantäne bzw. eines nicht verifizierten Entwicklers sperren.
Erst nach erfolgreicher Prüfung Rechtsklick → Öffnen verwenden, ggf. unter
Datenschutz & Sicherheit zulassen, oder am entpackten Binary
`xattr -d com.apple.quarantine ./stromea-steuerbox-simulator` ausführen.
Unter Windows fehlt Authenticode; SmartScreen-Warnungen sind möglich.

See [distribution instructions](VERTEILUNG.md) for startup and data paths.
