// Offline UI copy checks shared by dictionary and rendered-DOM tests.
'use strict';
function duplicates035(text){
 const hits=[];for(const m of String(text).matchAll(/\(([A-Za-z0-9]{2,12})\)\s*\(\1\)/g))hits.push(m[0]);
 for(const m of String(text).matchAll(/(?<![\p{L}-])([\p{L}]{2,})[ \t]+\1(?![\p{L}-])/giu))hits.push(m[0]);
 for(const word of String(text).match(/[\p{L}]+/gu)||[])if(/([\p{L}]{4,})s?\1/iu.test(word))hits.push(word);
 return [...new Set(hits)];
}
const forbidden035=/Klicke in ein Feld für|Fokussiere ein Feld für|Erklärung, Einheit und Herkunft|Quellen zu diesen Einstellungen|Stromea · Testvorschläge|Ausgangswert|Testvorschlag/iu;
const stale035=/(?<!\p{L})(?:frisch\p{L}*|quittiert\p{L}*|complete|Langtest|Wiederkehr|research\s+\d+)(?!\p{L})/iu;
const localizedStale035={en:/\bfresh(?:ly)?\b|\backnowledged\b/i,pl:/(?<!\p{L})śwież\p{L}*/iu,ru:/(?<!\p{L})свеж\p{L}*/iu,uk:/(?<!\p{L})свіж\p{L}*/iu,tr:/\btaze\p{L}*/iu,ro:/\bproasp[aă]t\p{L}*/iu};
function forbiddenCopy035(text,lang='de'){return stale035.test(text) || localizedStale035[lang]?.test(text) || (lang==='de' && forbidden035.test(text));}
module.exports={duplicates035,forbidden035,forbiddenCopy035};
