# Sicherheit & Sicherheitsmeldungen (Security Policy)

Wir nehmen die Sicherheit unserer Software und deiner lokalen Netzwerke sehr ernst.

---

## Unterstützte Versionen

Wir stellen Sicherheitsupdates für die aktuelle Version der Reihe 1.0.x bereit:

| Version | Unterstützt |
|---|---|
| **1.0.x (ab 1.0.0)** | :white_check_mark: Ja |
| Ältere Entwicklungsstände | :x: Nein |

---

## Sicherheitslücke melden (Responsible Disclosure)

Falls du eine Sicherheitslücke im Stromea Steuerbox-Simulator entdeckst, melde diese bitte **nicht öffentlich über ein GitHub-Issue**.

Bitte sende deine Meldung direkt per E-Mail an:
**kontakt@stromea.de**

### Was deine Meldung enthalten sollte
- Eine kurze Beschreibung des Problems und der potenziellen Auswirkung.
- Schritte zur Reproduktion (Proof of Concept, Befehlszeilen oder Testdaten).
- Angaben zu Betriebssystem, Simulator-Version und betroffener Netzwerkumgebung.

### Bearbeitung
Wir prüfen die Meldung und stimmen Rückfragen sowie eine mögliche Veröffentlichung mit dir ab. Bitte sende zunächst eine Beschreibung und anonymisierte Reproduktionsschritte; keine privaten Schlüssel oder Kundendaten.

---

## Lokale Sicherheitsprinzipien der Software

- **Kein offener Server:** Die Weboberfläche lauscht standardmäßig ausschließlich auf dem lokalen Rechner (`127.0.0.1:33287`).
- **Gezielte Kommunikation:** Netzwerkverbindungen zu Geräten erfolgen nur nach ausdrücklicher Freigabe und Bestätigung des Sicherheitsschlüssels (SKI).
- **Dateirechte:** Private Identitätsdateien werden unter Unix mit Modus 0600 angelegt. Unter Windows ist der Schutz des Benutzerverzeichnisses über die vorhandenen Benutzer-ACLs zu prüfen; die Anwendung richtet keine eigenen ACLs ein.
- **Gerätezustand:** Testgrenzen und Failsafe-Werte werden geschrieben. Die Grenzaufhebung wird versucht und quittungsbasiert bewertet; ursprüngliche Failsafe-Werte werden vorab gelesen und beim Aufräumen einzeln zurückgestellt und rückgelesen. Bei fehlendem Nachweis: Notbetriebs-Werte am Gerät prüfen. Siehe [Sicherheitsgrenzen](docs/SICHERHEIT.md).
