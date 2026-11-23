# Stromea Steuerbox-Simulator 1.0.3

Release date / Veröffentlichungsdatum: 2026-11-24

## English

Fixed: after the automatic restore on restart, the simulator stayed connected
but did not know the device's control functions, so "Start check" reported
"The device reports no control function". The simulator now reads them on the
same connection; a check can start right away without reconnecting (reconnecting
can put a device into failsafe).

A browser that remembers practice mode switches back to the simulator before it
offers a practice check, so a practice check never runs on a restored real device.

Downloads, provenance and verification are unchanged from 1.0.0.

## Deutsch

Behoben: Nach der automatischen Wiederherstellung beim Start war der Simulator
verbunden, kannte aber die Steuerfunktionen des Geräts nicht („Das Gerät meldet
keine Steuerfunktion“). Jetzt liest er sie in derselben Verbindung; die Prüfung
startet sofort, ohne neues Verbinden.

Ein Browser im Übungsmodus schaltet zuerst auf den Simulator zurück; eine Übung
läuft nie gegen ein wiederhergestelltes reales Gerät.

## Verify / Prüfen

```sh
gh attestation verify SHA256SUMS --repo stromea-de/steuerbox-simulator
gh attestation verify stromea-steuerbox-simulator_1.0.3_darwin_arm64.tar.gz --repo stromea-de/steuerbox-simulator
# If ALL eleven downloads are present / wenn ALLE elf Downloads vorliegen:
shasum -a 256 -c SHA256SUMS
# For just the selected download / bei nur einem Download:
grep '  stromea-steuerbox-simulator_1.0.3_darwin_arm64.tar.gz$' SHA256SUMS | shasum -a 256 -c -
```

For another platform, substitute its exact asset name; Linux can use `sha256sum`.
On Windows, verify provenance with `gh attestation verify` for SHA256SUMS and the
ZIP, then compare `Get-FileHash .\stromea-steuerbox-simulator_1.0.3_windows_amd64.zip -Algorithm SHA256`
with its entry in SHA256SUMS. Stop on any verification failure.

macOS: no Apple Developer ID signature or notarization; Windows: no Authenticode
signature. See the [1.0.0 release notes](RELEASE_NOTES_1.0.0.md) and the
[distribution instructions](VERTEILUNG.md) for opening and startup details.
