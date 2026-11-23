# Stromea Steuerbox-Simulator 1.0.2

Release date / Veröffentlichungsdatum: 2026-11-23

## English

The device's original values are now protected technically. Before an on-site
check writes anything, the simulator reads the device's failsafe values (power,
minimum duration) and its power limits fresh from the device and stores them per
device. At the end it writes them back and reads them back; no limit stays
active. If the simulator is cancelled, crashes or is killed, it restores the
original values automatically on the next start. Only values that differ are
written; an unchanged device is left alone. One device can be checked by only
one simulator window at a time. A status line shows whether the original values
were restored.

Also fixed: devices that reject new failsafe values while in failsafe state, or
that share one failsafe minimum duration between LPC and LPP, are restored
reliably.

Downloads, provenance and verification are unchanged from 1.0.0: six portable
archives, four Linux packages, source archive and SHA256SUMS, each covered by a
GitHub Artifact Attestation.

## Deutsch

Die Originalwerte des Geräts werden technisch gesichert: frisch gelesen, je
Gerät gespeichert, am Ende zurückgeschrieben und zurückgelesen; nach Abbruch
oder Absturz automatisch beim nächsten Start wiederhergestellt.

## Verify / Prüfen

```sh
gh attestation verify SHA256SUMS --repo stromea-de/steuerbox-simulator
gh attestation verify stromea-steuerbox-simulator_1.0.2_darwin_arm64.tar.gz --repo stromea-de/steuerbox-simulator
# If ALL eleven downloads are present / wenn ALLE elf Downloads vorliegen:
shasum -a 256 -c SHA256SUMS
# For just the selected download / bei nur einem Download:
grep '  stromea-steuerbox-simulator_1.0.2_darwin_arm64.tar.gz$' SHA256SUMS | shasum -a 256 -c -
```

For another platform, substitute its exact asset name; Linux can use `sha256sum`.
On Windows, verify provenance with `gh attestation verify` for SHA256SUMS and the
ZIP, then compare `Get-FileHash .\stromea-steuerbox-simulator_1.0.2_windows_amd64.zip -Algorithm SHA256`
with its entry in SHA256SUMS. Stop on any verification failure.

macOS: no Apple Developer ID signature or notarization; Windows: no Authenticode
signature. See the [1.0.0 release notes](RELEASE_NOTES_1.0.0.md) and the
[distribution instructions](VERTEILUNG.md) for opening and startup details.
