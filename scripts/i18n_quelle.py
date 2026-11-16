#!/usr/bin/env python3
"""040: Herkunft jeder Übersetzung (Prüfsumme des deutschen Quelltexts).

Für jede Fremdsprache steht in `internal/webui/i18n-quelle.json` je Schlüssel die
SHA-256-Prüfsumme (16 Hex-Zeichen) des de-Werts, aus dem die Übersetzung entstand.
Ändert sich der de-Wert, passt die Prüfsumme nicht mehr und `test_i18n040.py` wird rot.

  python3 scripts/i18n_quelle.py --offen en        # veraltete Schlüssel auflisten (JSON)
  python3 scripts/i18n_quelle.py --bestaetigen en KEYDATEI
      # Übersetzungen der in KEYDATEI (JSON-Liste) genannten Schlüssel als aktuell bestätigen
Nur Standardbibliothek. Die Datei liegt außerhalb von assets/ und wird nicht ausgeliefert.
"""
import hashlib, json, sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
BASE = ROOT / 'internal/webui/assets/i18n'
QUELLE = ROOT / 'internal/webui/i18n-quelle.json'
SPRACHEN = ['en', 'pl', 'ru', 'uk', 'tr', 'ro']


def pruefsumme(text):
    return hashlib.sha256(text.encode('utf-8')).hexdigest()[:16]


def laden():
    de = json.loads((BASE / 'de.json').read_text())
    quelle = json.loads(QUELLE.read_text()) if QUELLE.exists() else {}
    return de, quelle


def veraltet(de, quelle, sprache):
    eintraege = quelle.get(sprache, {})
    return sorted(k for k, v in de.items() if eintraege.get(k) != pruefsumme(v))


def ueberzaehlig(de, quelle, sprache):
    return sorted(set(quelle.get(sprache, {})) - set(de))


def speichern(quelle):
    QUELLE.write_text(json.dumps({s: dict(sorted(quelle[s].items())) for s in SPRACHEN if s in quelle},
                                 ensure_ascii=False, indent=0, sort_keys=False) + '\n')


if __name__ == '__main__':
    args = sys.argv[1:]
    de, quelle = laden()
    if len(args) == 2 and args[0] == '--offen':
        print(json.dumps(veraltet(de, quelle, args[1]), ensure_ascii=False, indent=1))
    elif len(args) == 3 and args[0] == '--bestaetigen' and args[1] in SPRACHEN:
        keys = json.loads(Path(args[2]).read_text())
        unbekannt = [k for k in keys if k not in de]
        if unbekannt:
            raise SystemExit('Unbekannte Schlüssel: ' + json.dumps(unbekannt[:5], ensure_ascii=False))
        eintraege = quelle.setdefault(args[1], {})
        for k in keys:
            eintraege[k] = pruefsumme(de[k])
        for k in ueberzaehlig(de, quelle, args[1]):
            del eintraege[k]
        speichern(quelle)
        print(f'{args[1]}: {len(keys)} Schlüssel bestätigt, {len(veraltet(de, quelle, args[1]))} veraltet')
    else:
        raise SystemExit(__doc__)
