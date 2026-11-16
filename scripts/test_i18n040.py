#!/usr/bin/env python3
"""040 (Review Fable, Strukturbefund): Übersetzungen müssen auf dem aktuellen de-Wert beruhen.

Jede Fremdsprache führt je Schlüssel die Prüfsumme des de-Werts, aus dem sie übersetzt wurde
(`internal/webui/i18n-quelle.json`, gepflegt mit `scripts/i18n_quelle.py`). Ändert jemand einen
de-Wert, ohne die sechs Übersetzungen nachzuziehen und zu bestätigen, ist dieser Test rot.
Zusätzlich: Glossar-Stichproben aus REVIEW_040_FABLE (T-01, T-05, T-06, T-09 …).
"""
import json, sys
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parent))
from i18n_quelle import BASE, SPRACHEN, laden, veraltet, ueberzaehlig

de, quelle = laden()
fehler = []
for s in SPRACHEN:
    alt = veraltet(de, quelle, s)
    extra = ueberzaehlig(de, quelle, s)
    if alt:
        fehler.append(f'{s}: {len(alt)} Übersetzungen beruhen nicht auf dem aktuellen de-Wert, z. B. {alt[:3]}')
    if extra:
        fehler.append(f'{s}: {len(extra)} Prüfsummen ohne Schlüssel, z. B. {extra[:3]}')

d = {s: json.loads((BASE / f'{s}.json').read_text()) for s in SPRACHEN}
titel = next(k for k in de if k.startswith('Inbetriebnahmeprotokoll · '))
erwartet = {  # Kernbegriffe, die im Review falsch waren (Teilstrings, kleingeschrieben verglichen)
    ('en', titel): 'commissioning report', ('pl', titel): 'protokół uruchomienia',
    ('ro', titel): 'punere în funcțiune', ('ru', titel): 'ввода в эксплуатацию',
    ('tr', titel): 'devreye alma', ('uk', titel): 'введення в експлуатацію',
    ('en', 'Grenze'): 'control active', ('tr', 'Grenze'): 'kontrol etkin',
    ('en', 'Funktionen'): 'functions', ('en', 'Kein Gerät gefunden'): 'no device found',
}
for (s, k), teil in erwartet.items():
    if k in d[s] and teil not in d[s][k].lower():
        fehler.append(f'{s}: „{k[:40]}“ enthält nicht „{teil}“: {d[s][k][:60]!r}')
verboten = {'ro': ['rezerv'], 'tr': ['besleme'], 'pl': ['awari']}  # T-06, T-09, T-08 ('awari' trifft awaria/awarii, nicht 'awaryjny')
notbetrieb = [k for k, v in de.items() if ('Notbetrieb' in v or 'Ausfallfrist' in v or 'Einspeis' in v)
              and 'eserve' not in v and 'Versorg' not in v and 'Defekt' not in v]
for s, woerter in verboten.items():
    for k in notbetrieb:
        for w in woerter:
            if w in d[s][k].lower():
                fehler.append(f'{s}: „{k[:40]}“ enthält „{w}“: {d[s][k][:60]!r}')

# 044: Übersetzungen durch zwei KI-Reviewer geprüft; kein Hinweis „ungeprüft“ in der Oberfläche
ROOT = BASE.parent.parent.parent.parent
HINWEIS = ['translation-status', 'Übersetzung ungeprüft', 'Translation not reviewed', 'Tłumaczenie niesprawdzone',
           'Перевод не проверен', 'Переклад не перевірено', 'Çeviri kontrol edilmedi', 'Traducere neverificată']
files = ['internal/webui/assets/index.html', 'internal/webui/assets/protokoll.html', 'internal/webui/assets/i18n.js',
         'internal/webui/assets/i18n-data.js', 'internal/webui/assets/app.js']
# The standalone prototype belongs only to the private checkout (same boundary as check.sh).
if (ROOT / 'scripts/build-prototyp.py').exists():
    files.append('docs/prototyp/steuerbox-simulator-prototyp.html')
for f in files:
    text = (ROOT / f).read_text()
    for h in HINWEIS:
        if h in text:
            fehler.append(f'{f}: Hinweis „Übersetzung ungeprüft“ ist noch vorhanden ({h})')
# R044-02: Zählformen für pl/ru/uk (Intl.PluralRules in app.js): „many“ hat eine eigene Form
for s, viele in {'pl': 'znalezionych urządzeń', 'ru': 'устройств найдено', 'uk': 'пристроїв знайдено'}.items():
    if d[s]['Geräte gefunden (viele)'] != viele or d[s]['Geräte gefunden'] == viele:
        fehler.append(f'{s}: Zählform „Geräte gefunden (viele)“ falsch: {d[s]["Geräte gefunden (viele)"]!r}')
for s in ['de'] + SPRACHEN:
    w = json.loads((BASE / f'{s}.json').read_text()) if s == 'de' else d[s]
    if 'Übersetzung ungeprüft' in w:
        fehler.append(f'{s}: Schlüssel „Übersetzung ungeprüft“ ist noch vorhanden')

if fehler:
    print('040 i18n-Herkunft: FEHLER')
    for f in fehler[:40]:
        print(' -', f)
    print(f'({len(fehler)} Fehler)')
    raise SystemExit(1)
print(f'040 i18n-Herkunft: {len(de)} Schlüssel × {len(SPRACHEN)} Sprachen beruhen auf dem aktuellen de-Wert')
