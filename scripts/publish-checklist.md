# Veröffentlichung 1.0.0 vorbereiten

Stand: 16.11.2026. Nicht ausgeführt. Diese Checkliste beschreibt spätere,
ausdrücklich freizugebende Schritte; der lokale Buildwrapper lädt nichts hoch.

1. Im privaten Entwicklungsbaum den Allowlist-Export mit `scripts/export_public.py`
   in ein neues Ziel außerhalb des Quellbaums erzeugen und prüfen. Nur der Export
   wird öffentlich; keine private Historie, keine Übernahme des privaten `.git`.
2. Im Export ein frisches Git-Repository anlegen (Exportoption `--init-git` setzt
   die Stromea-Identität). Dateien prüfen und als einzigen initialen Commit
   „Stromea Steuerbox-Simulator 1.0.0“ erfassen. Historie mit `git log` und
   Dateiliste mit `git ls-files` prüfen; Secretscan und Tests ausführen.
3. Das Ziel `stromea-de/steuerbox-simulator` durch den Verantwortlichen prüfen
   bzw. bereitstellen lassen. Keine bestehende abweichende Historie überschreiben,
   keine private Historie pushen, keinen Force-Push verwenden. Sichtbarkeit für
   den eigentlichen Release öffentlich; Actions und Attestations müssen erlaubt sein.
4. Erst nach separater Freigabe den geprüften initialen Commit übertragen.
   Optional zunächst einen privaten Staging-Export mit identischem Workflow am
   Standardbranch per `workflow_dispatch`, Eingabe `mode=build-only`, prüfen.
   Dieser Lauf baut ohne Attestations und ohne Release; Artefakte sieben Tage.
5. Release Notes und README auf 1.0.0, reale Assetnamen, fehlende Betriebssystem-
   Signaturen und Verifikationsbefehle prüfen. Go 1.27.3 / GoReleaser OSS 2.20.0.
   Für einen Entwurf vor dem Tag die Repositoryvariable `RELEASE_DRAFT=true`
   setzen; Standard ohne diese Variable ist direkte Veröffentlichung nach grünen
   Prüfungen. Keine Beta und keine Vorabversion.
6. Erst am freigegebenen Veröffentlichungstag den sauberen Commit mit `v1.0.0`
   taggen und den Tag übertragen. Bereits vorhandene Tags nicht verschieben.
   Der Tag-Push startet `.github/workflows/release.yml`. Dieser prüft Go-Tests,
   baut sechs Plattformarchive, vier Linux-Pakete und das Quellarchiv, erstellt
   die finale `SHA256SUMS` ohne Paketmanagerentwürfe und attestiert alle zwölf
   Dateien. Der Releasejob ist auf das öffentliche Zielrepository beschränkt.
7. Der Workflow prüft alle zwölf Attestations vor `gh release create` mit
   `--verify-tag`, Titel „Stromea Steuerbox-Simulator 1.0.0“ und
   `docs/RELEASE_NOTES_1.0.0.md`. Keine zusätzlichen Assets, kein minisign und
   kein cosign-Bundle. Bei einem Fehler anhalten. Wiederholung nach bereits
   erfolgter Releaseerstellung scheitert absichtlich; nichts automatisch ersetzen.
8. Bei Entwurfsmodus Assetliste und Attestations prüfen und erst danach manuell
   veröffentlichen. Bei direktem Modus anschließend alle elf Downloads prüfen.
   Website-Go-live und Paketmanageraufnahme benötigen eigene Freigaben.

Nach Download vor Start:

```sh
gh attestation verify SHA256SUMS --repo stromea-de/steuerbox-simulator
gh attestation verify stromea-steuerbox-simulator_1.0.0_darwin_arm64.tar.gz --repo stromea-de/steuerbox-simulator
# In einem Verzeichnis mit allen elf Downloads:
shasum -a 256 -c SHA256SUMS
```

Bei einem einzelnen Download nur dessen exakten Eintrag prüfen, siehe
[Verteilung](../docs/VERTEILUNG.md). macOS ist nicht notarisiert und ohne
Developer-ID-Signatur: Quarantäne/„nicht verifizierter Entwickler“ kann blockieren.
Erst nach erfolgreicher Prüfung Rechtsklick → Öffnen, ggf. Datenschutz & Sicherheit,
oder `xattr -d com.apple.quarantine ./stromea-steuerbox-simulator` verwenden.
Windows hat keine Authenticode-Signatur; SmartScreen-Warnungen sind möglich.
