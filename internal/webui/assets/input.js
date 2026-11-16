/* Shared input parsing: validation and sending use these exact values. */
'use strict';
globalThis.StromeaInput = {
  number(text, min = 0, max = 1000000, unit = '') {
    const raw = String(text).trim();
    if (/^[1-9]\d{0,2}\.\d{3}$/.test(raw)) throw new Error('Mehrdeutiger Punkt: Tausenderzeichen weglassen (4200). Dezimalstellen mit Komma eingeben.');
    if (!/^\d+(?:[.,]\d{1,3})?$/.test(raw)) throw new Error('Zahl mit höchstens drei Nachkommastellen eingeben (Komma oder Punkt).');
    const value = Number(raw.replace(',', '.'));
    // 039 (QA-14): local number format and unit (de: 0,001 W … 1.000.000 W).
    const i18n = globalThis.StromeaI18n, format = v => v.toLocaleString(i18n?.locale?.() || 'de-DE', {maximumFractionDigits: 3}) + (unit ? ' ' + unit : '');
    if (!Number.isFinite(value) || value < min || value > max) throw new Error((i18n?.t || (x => x))('Wert zwischen {min} und {max} eingeben.').replace('{min}', format(min)).replace('{max}', format(max)));
    return value;
  },
  // 041 (Nutzer 2026-11-14): Watt-Felder zeigen das Tausendertrennzeichen der UI-Sprache (Intl). Gelesen wird
  // nur, was sich eindeutig in Ziffern, Gruppen- und Dezimalzeichen dieser Sprache zerlegen lässt; alles
  // andere bleibt unverändert stehen und wird von der Prüfung benannt. Protokoll und PDF bleiben deutsch.
  separators(locale) {
    const parts = new Intl.NumberFormat(locale, {useGrouping: true}).formatToParts(1234567.5);
    return {group: parts.find(p => p.type === 'group')?.value || '', decimal: parts.find(p => p.type === 'decimal')?.value || '.'};
  },
  wattsParts(text, locale) {
    const {group, decimal} = this.separators(locale), space = /[\s\u00a0\u202f]/;
    let int = '', frac = '', dec = false;
    for (const ch of String(text).trim()) {
      if (/[0-9]/.test(ch)) { if (dec) frac += ch; else int += ch; continue; }
      if (ch === decimal && !dec) { dec = true; continue; }
      if (!dec && (ch === group || space.test(group) && space.test(ch))) continue;
      return null;
    }
    return {int, frac, dec};
  },
  wattsFormat(text, locale, caret = String(text).length) {
    const raw = String(text), p = this.wattsParts(raw, locale), {decimal} = this.separators(locale);
    if (!p || !p.int || p.int.length > 9 || /^0\d/.test(p.int)) return {text: raw, caret};
    const out = Number(p.int).toLocaleString(locale, {useGrouping: true, maximumFractionDigits: 0}) + (p.dec ? decimal + p.frac : '');
    let significant = 0;
    for (const ch of raw.slice(0, caret)) if (/[0-9]/.test(ch) || ch === decimal) significant++;
    let at = 0;
    for (let seen = 0; at < out.length && seen < significant; at++) if (/[0-9]/.test(out[at]) || out[at] === decimal) seen++;
    return {text: out, caret: at};
  },
  watts(text, locale, min = 0) {
    const p = this.wattsParts(text, locale);
    return this.number(p && (p.int || p.frac) ? (p.int || '0') + (p.dec ? ',' + p.frac : '') : String(text), min, 1000000, 'W');
  },
  wattsConvert(text, from, to) {
    const p = this.wattsParts(text, from);
    if (!p || !p.int) return String(text);
    return this.wattsFormat(p.int + (p.dec ? this.separators(to).decimal + p.frac : ''), to).text;
  },
  // 039 (QA-09/10): only separators are removed. Invalid characters and excess length stay visible and are reported.
  ski(text) { return String(text).replace(/[\s:-]/g, '').toUpperCase().match(/.{1,4}/g)?.join(' ') || ''; },
  skiProblem(text) {
    const compact = String(text).replace(/[\s:-]/g, '').toUpperCase(), invalid = [...new Set(compact.replace(/[0-9A-F]/g, ''))];
    if (invalid.length) return {kind: 'invalid', chars: invalid.join(' ')};
    if (compact.length > 40) return {kind: 'long', count: compact.length};
    if (compact.length < 40) return {kind: 'short', count: compact.length};
    return null;
  },
  dictation(text) {
    const words = {null:'0',eins:'1',ein:'1',zwei:'2',drei:'3',vier:'4',fünf:'5',sechs:'6',sieben:'7',acht:'8',neun:'9',a:'A',ah:'A',anton:'A',b:'B',be:'B',berta:'B',c:'C',ze:'C',zeh:'C',cäsar:'C',caesar:'C',d:'D',de:'D',dora:'D',e:'E',emil:'E',f:'F',eff:'F',friedrich:'F'};
    const tokens = String(text).split(/[\s,;.!?]+/).filter(Boolean).map(raw => ({raw, value: words[raw.toLowerCase()] ?? (/^[0-9]$/.test(raw) ? raw : null)}));
    return {hex:tokens.map(t=>t.value || '').join(''), unknown:tokens.filter(t=>t.value===null).map(t=>t.raw), tokens};
  },
  ship(text) {
    const raw = String(text).trim();
    if (raw.length > 4096 || !raw.startsWith('SHIP;') || !raw.endsWith('ENDSHIP;')) throw new Error('Füge den vollständigen Text aus dem QR-Code des Geräts ein.');
    const values = {};
    for (const part of raw.slice(5, -8).split(';')) {
      if (!part) continue;
      const i = part.indexOf(':');
      if (i < 1) throw new Error('Ungültiger SHIP-Eintrag.');
      const key = part.slice(0, i), value = part.slice(i + 1).trim();
      if (Object.hasOwn(values, key)) throw new Error('Doppelter SHIP-Eintrag: ' + key);
      values[key] = value;
    }
    const ski = (values.SKI || '').replace(/[\s:-]/g, '').toLowerCase();
    if (!/^[0-9a-f]{40}$/.test(ski) || !values.ID) throw new Error('SHIP-QR benötigt eine gültige SKI und ID.');
    if (values.PIN) throw new Error('Dieses Gerät benötigt eine SHIP-PIN. PIN-Kopplung wird noch nicht unterstützt.');
    return {ski, id: values.ID, brand: values.BRAND || '', model: values.MODEL || '', serial: values.SERIAL || '', type: values.TYPE || '', category: values.CAT || ''};
  },
  ownShip(ski, id) {
    const normalized = ski.replace(/[\s:-]/g, '').toLowerCase();
    if (!/^[0-9a-f]{40}$/.test(normalized)) throw new Error('Ungültige eigene SKI.');
    const grouped = normalized.toUpperCase().match(/.{4}/g).join(' ');
    return 'SHIP;SKI:' + grouped + ';ID:' + (id || 'Stromea-EnergyGuard-' + normalized.slice(0, 8)) + ';BRAND:Stromea;TYPE:Generic;MODEL:Steuerbox-Simulator;ENDSHIP;';
  }
};
