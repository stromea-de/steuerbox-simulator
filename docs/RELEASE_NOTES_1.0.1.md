# Stromea Steuerbox-Simulator 1.0.1

Release date / Veröffentlichungsdatum: 2026-11-19

## English

Bug fix for the on-site check of fallback mode (failsafe, LPC and LPP).
Version 1.0.0 could report "failsafe effect passed" right after the heartbeat
stopped, before the device's heartbeat timeout had expired. If the power was
already below the failsafe value under the active limit, a device that never
entered failsafe could pass.

Now the failsafe effect is only evaluated after the device's heartbeat timeout:
on real devices at least 120 s (LPC/LPP 1.0.0) after the heartbeat stop or a
later acknowledged heartbeat, followed by the 30 s observation reserve; a
shorter configured value no longer shortens it. A reading at or below the
failsafe value only counts if the power was clearly above it when the heartbeat
stopped, or if the device itself reports failsafe; a repeated measurement with
an old device timestamp does not count. Otherwise the step stays "not checked"
("fallback mode not distinguishable"), the return steps (also in the
long-duration test) are "not checked", and the failsafe line in the power chart
is shown light dotted ("not verified"). Practice mode is unchanged. Reports
created with 1.0.0 are not re-evaluated; repeat an affected on-site check with
1.0.1.

Downloads, provenance and verification are unchanged from 1.0.0: six portable
archives, four Linux packages, source archive and SHA256SUMS, each covered by a
GitHub Artifact Attestation.

## Deutsch

Fehlerbehebung: Der Notbetrieb (Failsafe) wird erst nach Ablauf der Ausfallfrist
bewertet; mit 1.0.0 vor Ort erstellte Notbetriebsnachweise bitte mit 1.0.1
wiederholen.

## Verify / Prüfen

```sh
gh attestation verify SHA256SUMS --repo stromea-de/steuerbox-simulator
gh attestation verify stromea-steuerbox-simulator_1.0.1_darwin_arm64.tar.gz --repo stromea-de/steuerbox-simulator
# If ALL eleven downloads are present / wenn ALLE elf Downloads vorliegen:
shasum -a 256 -c SHA256SUMS
# For just the selected download / bei nur einem Download:
grep '  stromea-steuerbox-simulator_1.0.1_darwin_arm64.tar.gz$' SHA256SUMS | shasum -a 256 -c -
```

For another platform, substitute its exact asset name; Linux can use `sha256sum`.
On Windows, verify provenance with `gh attestation verify` for SHA256SUMS and the
ZIP, then compare `Get-FileHash .\stromea-steuerbox-simulator_1.0.1_windows_amd64.zip -Algorithm SHA256`
with its entry in SHA256SUMS. Stop on any verification failure.

macOS: no Apple Developer ID signature or notarization; Windows: no Authenticode
signature. See the [1.0.0 release notes](RELEASE_NOTES_1.0.0.md) and the
[distribution instructions](VERTEILUNG.md) for opening and startup details.
