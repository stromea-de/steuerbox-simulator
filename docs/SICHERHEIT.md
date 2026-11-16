# Betrieb und Nachweisgrenzen

Stand: 16.11.2026. Der Stromea Steuerbox-Simulator dokumentiert Kommunikations- und Funktionstests im jeweiligen Prüfaufbau. Er ist keine Steuerbox-Hardware, beansprucht keine Zertifizierung und ersetzt keine Konformitätsprüfung.

## Üben und Gerätebetrieb

Im Übungsmodus laufen Steuerbox und Testgerät über SHIP/TLS/SPINE auf Loopback. Eine bestandene Übung belegt keinen Test eines realen Geräts.

Der Gerätebetrieb ist eine ausdrückliche Auswahl. Suche nur auf der gewählten Netzwerkkarte; die Anwendung löst keine Gerätenamen auf und nutzt keinen Proxy. mDNS-Namen, Modellangaben und angekündigte Schlüssel sind unbestätigte Angaben. Vor dem Verbinden die Geräte-SKI am Gerät vergleichen und die eigene SKI auf der Gegenstelle freigeben. Kein automatisches Vertrauen aus Suchergebnissen. Die Verbindung wird auf das bestätigte IP-Literal und den Zertifikatsschlüssel festgelegt. Geräte, die zwingend eine eigene eingehende SHIP-Verbindung oder die Ankündigung des Simulators erwarten, benötigen einen gesonderten Interoperabilitätsnachweis. Der SHIP-Listener bleibt auf Loopback.

Die Suche nutzt das erste geeignete IPv4-Subnetz der gewählten Karte. IPv6-only-Suche, VLAN-Weiterleitung und eine Firewall-Einrichtung sind nicht enthalten. Eine leere Geräteliste beweist keine fehlende EEBUS-Unterstützung. Nur mit ausdrücklicher Freigabe des zuständigen Betreibers auf reale Geräte zugreifen; Testwerte und Auswirkungen vorher prüfen.

## Schreiben, Wartezeit und Freigabe

Die Geräteprüfung sendet Leistungsgrenzen und schreibt Failsafe-Werte. Ursprüngliche Failsafe-Einstellungen werden vor Änderungen gelesen und beim Aufräumen mit eigenen Fristen zurückgeschrieben und frisch rückgelesen. Ohne lesbare Ursprungswerte wird die Failsafe-Konfiguration nicht überschrieben. Fehlendes Rücklesen oder Rückschreibfehler: Notbetriebs-Werte am Gerät prüfen. Voreinstellungen von 4.200 W Bezug, 3.000 W Einspeisung und 500 W Failsafe sind Testwerte; ihre Eignung für den Aufbau muss die Fachkraft prüfen.

Die Standardprüfung prüft den Eintritt in den Notbetrieb nach Heartbeat-Ausfall, mit höchstens 120 s Ausfall-Erkennung je Richtung. Die Wiederkehr nach Mindestdauer bleibt als „nicht geprüft (Langtest)“ ausgewiesen. Nur der optionale Langtest wartet mindestens zwei Stunden je beobachtbarer Leistungsrichtung. Nach dem Ausfalltest kann das Gerät bis zur eingestellten Mindestdauer im Notbetrieb bleiben; ein neues Lebenszeichen und Aufheben versucht eine frühere Rückkehr, deren Wirkung gesondert gemessen wird. Im Übungsprofil gelten kurze Simulatorzeiten. Daraus entsteht kein unabhängiger Nachweis normativer Mindestzeiten.

Auch bei Abbruch versucht die Anwendung, aktive Testgrenzen aufzuheben. Sie verwendet eigene Zeitlimits und versucht bei Bedarf die Wiederverbindung zum unverändert bestätigten Gerät. Erst eine positive Gerätequittung bestätigt die Freigabe. Bei „Freigabe nicht bestätigt“ können Grenzen weiter gelten: Anwendung bis zum Ende der Freigabeversuche geöffnet lassen und den Gerätezustand vor Ort kontrollieren und gegebenenfalls freigeben. Die endliche Gültigkeit aktiver Grenzen ersetzt diese Kontrolle wegen möglicher Failsafe-Zustände nicht.

## Bericht und Daten

„Bestanden“ verlangt die vorgesehenen Nachweise. Fehlende Funktionen, mehrdeutige Messwerte oder zu geringe Ausgangsleistung ergeben ausdrücklich „nicht geprüft“ und gegebenenfalls „Unvollständig“. Messwerte sind Gerätemeldungen, keine unabhängige elektrische Messung. Hersteller-/Firmwareangaben werden übernommen, soweit per SPINE verfügbar.

Die lokale HTTP-Oberfläche ist auf Loopback beschränkt. Schreibende API-Aufrufe prüfen Sitzungstoken, Host und Origin. Identitäten und Vertrauensdaten liegen im Benutzer-Konfigurationsverzeichnis. Unter Unix werden private Identitätsdateien mit Modus 0600 angelegt; unter Windows die vorhandenen Benutzer-ACLs kontrollieren. Kernprüfung und Berichtserstellung funktionieren lokal ohne Cloud-Dienst oder Benutzerkonto; die Anwendung enthält keine eigene Telemetrie. Optionales Browserdiktat kann nach ausdrücklicher Zustimmung für die Sitzung Audio an den Spracherkennungsdienst des Browsers senden und eine Internetverbindung benötigen. Für die Kernprüfung und Berichte ist das Diktat nicht erforderlich. Links zu Website oder Hilfe öffnen externe Seiten erst bei Nutzung.

PDF und JSON können personenbezogene Betriebs-/Prüferangaben, Gerätekennungen und Schlüsselkennungen enthalten. Vor Weitergabe anonymisieren. Die Berichtsübersicht ist kein dauerhaftes Archiv: Ergebnisse vor dem nächsten Lauf oder Programmende speichern. Aufbewahrung und Zugriffsschutz liegen beim Nutzer. Sicherheitsmeldungen: [SECURITY.md](../SECURITY.md).

## Lizenz und Marken

Ausschließlich **AGPL-3.0-only**, ohne kommerzielles Dual-Licensing; permissive Fremdabhängigkeiten mit vollständigen Hinweisen in [LIZENZEN.md](LIZENZEN.md).

Die EEBUS-Wort-/Bildmarke ist eine eingetragene Marke der EEBus Initiative e.V. EEBUS bezeichnet hier den unterstützten Kommunikationsstandard. Der Stromea Steuerbox-Simulator ist ein unabhängiges Open-Source-Projekt und wird weder von der EEBus Initiative e.V. noch von der EEBUService GmbH herausgegeben oder zertifiziert. Er ist keine Steuerbox-Hardware und ersetzt keine Konformitätsprüfung.

Grundlage dieser Beschreibung: Code und lokale Tests dieser Version, erneut für Lokalität, Browserdiktat und Ergebniswortlaut gelesen am 16.11.2026. Sie beschreibt den implementierten Umfang, keine allgemeine Gerätekompatibilität.
