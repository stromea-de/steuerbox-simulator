// Offline boundary regression including the actual embedded MIT QR codec.
'use strict';
const fs = require('fs'), vm = require('vm'), assert = require('assert/strict');
const ctx = vm.createContext({TextEncoder, TextDecoder, Uint8Array, Uint8ClampedArray});
for (const name of ['qr-vendor.js','input.js']) vm.runInContext(fs.readFileSync('internal/webui/assets/' + name, 'utf8'), ctx);
const input = ctx.StromeaInput, qr = ctx.StromeaQR;
assert.equal(input.number('4200,125'), 4200.125);
assert.equal(input.number('4200.125'), 4200.125);
for (const raw of ['', 'NaN', 'Infinity', '-1', '1000001','1.234,56','1,2,3','1e6','0x20','1.0001']) assert.throws(() => input.number(raw), raw);
const ski = '0123456789abcdef0123456789abcdef01234567';
const text = input.ownShip(ski,'actual-id');
assert.equal(input.ship(text).ski,ski);
assert.equal(input.ship(text).id,'actual-id');
assert.equal(input.ship(text.replace('ENDSHIP;', 'X-UNKNOWN:more:stuff;ENDSHIP;')).ski,ski);
for (const raw of ['bad', 'SHIP;SKI:bad;ID:id;ENDSHIP;', text.replace('ID:actual-id;',''), text.replace('ENDSHIP;','SKI:'+ski+';ENDSHIP;'),text.replace('ENDSHIP;','PIN:1234;ENDSHIP;')]) assert.throws(() => input.ship(raw));
const raw=qr.encode(text,'raw',{ecc:'medium',border:4,scale:4});
const bitmap = new qr.Bitmap({width:raw[0].length,height:raw.length});bitmap.data=raw;
assert.equal(qr.decode(bitmap.toImage()), text);
console.log('Input boundaries: numeric/SHIP boundaries and offline QR encode/decode passed');
for (const raw of ['4.200','12.345','123.456']) assert.throws(() => input.number(raw), raw);
// 039 (QA-09/10): separators are removed; invalid characters and excess length stay visible and are reported.
assert.equal(input.ski('abCD-0123:45!zz'), 'ABCD 0123 45!Z Z');
assert.equal(JSON.stringify(input.skiProblem('abCD-0123:45!zz')), JSON.stringify({kind:'invalid', chars:'! Z'}));
assert.equal(JSON.stringify(input.skiProblem('A'.repeat(45))), JSON.stringify({kind:'long', count:45}));
assert.equal(JSON.stringify(input.skiProblem('A'.repeat(12))), JSON.stringify({kind:'short', count:12}));
assert.equal(input.skiProblem('0123 4567 89ab cdef 0123 4567 89ab cdef 0123 4567'), null);
for (const [spoken, hex] of [['null eins zwei drei','0123'],['A Be Ze De','ABCD'],['Ah Berta Cäsar Friedrich','ABCF'],['Anton Dora Emil Eff','ADEF'],['4 5 sechs sieben','4567']]) {
 const parsed=input.dictation(spoken); assert.equal(parsed.hex,hex); assert.equal(parsed.unknown.length,0);
}
const unknown=input.dictation('Anton Apfel zehn'); assert.equal(unknown.hex,'A'); assert.equal(unknown.unknown.join(','),'Apfel,zehn');
console.log('015: unambiguous numbers, SKI mask and German dictation parser passed');
// 041 (Nutzer 2026-11-14): Watt-Felder mit automatischem Tausendertrennzeichen je Sprache; Caret bleibt hinter derselben Ziffer.
{
 const nb=' ';
 for (const [locale, typed, shown, value] of [
  ['de-DE','4200','4.200',4200],['de-DE','1000000','1.000.000',1e6],['de-DE','10900,5','10.900,5',10900.5],['de-DE','4.2001','42.001',42001],['de-DE','50','50',50],
  ['en-GB','4200','4,200',4200],['en-GB','10900.5','10,900.5',10900.5],
  ['tr-TR','4200','4.200',4200],['ro-RO','42000','42.000',42000]]) {
  const f = input.wattsFormat(typed, locale, typed.length);
  assert.equal(f.text, shown, locale + ' ' + typed);
  assert.equal(input.watts(f.text, locale), value, locale + ' parse ' + f.text);
 }
 for (const locale of ['pl-PL','ru-RU','uk-UA']) {
  const f = input.wattsFormat('42000', locale, 5);
  assert(/^42[\s  ]000$/.test(f.text), locale + ' groups with a space: ' + JSON.stringify(f.text));
  assert.equal(input.watts(f.text, locale), 42000);
  assert.equal(input.watts('42 000', locale), 42000, locale + ' typed space');
  assert.equal(input.watts('4200,5', locale), 4200.5, locale + ' decimal comma');
 }
 // Caret: after typing the 4th digit, the caret stays behind it.
 assert.equal(input.wattsFormat('4200', 'de-DE', 4).caret, 5);
 assert.equal(input.wattsFormat('420|0'.replace('|',''), 'de-DE', 3).caret, 4);
 // Invalid text is left untouched (validation names the problem), never silently changed.
 assert.equal(input.wattsFormat('42a0', 'de-DE', 4).text, '42a0');
 for (const raw of ['', '42a0', '1.000.001', '-5', '1,2345']) assert.throws(() => input.watts(raw, 'de-DE'), raw);
 assert.equal(input.watts('0,001', 'de-DE', 0.001), 0.001);
 // Re-formatting a value written in another language (language switch) keeps its number.
 assert.equal(input.wattsConvert('10.900,5', 'de-DE', 'en-GB'), '10,900.5');
 assert.equal(input.wattsConvert('10,900.5', 'en-GB', 'de-DE'), '10.900,5');
 assert.equal(input.wattsConvert('kaputt', 'en-GB', 'de-DE'), 'kaputt');
 void nb;
}
console.log('041 Watt-Felder: Tausendertrennzeichen je Sprache, Caret und Sprachwechsel bestanden');
