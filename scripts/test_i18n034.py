#!/usr/bin/env python3
"""Complete offline dictionary audit; DOM checks live in e2e_034.mjs."""
import json,re,os
from pathlib import Path
ROOT=Path(__file__).resolve().parent.parent
BASE=ROOT/'internal/webui/assets/i18n'
LANGS=['de','en','pl','ru','uk','tr','ro']
# Exact display values, with a reason; never allow arbitrary German sentences.
ALLOW={
 '0123 4567 89AB CDEF …':'synthetic identifier example',
 'Details':'shared English/German word', 'GitHub':'brand', 'Simulator':'shared term',
 'Stromea Steuerbox-Simulator':'product name',
 'Stromea Steuerbox-Simulator · AGPL-3.0-only':'product name and license identifier',
 'System':'shared English/Polish/German word','VPN':'technical acronym','Version':'shared English/German word',
 'W':'SI unit','{watts} W':'SI unit after a number (043)','optional':'shared English/German word (043)','{time} min':'time with SI unit (043)','WLAN':'network acronym','Ethernet (LAN)':'technical network term (040: translation of the current de value)','stromea.de/simulator':'product domain',
 'Firma':'shared Polish/Turkish/German company term','Profil':'shared Polish/Turkish/Romanian/German word','Prototyp':'shared Polish/German word'
}
def audit():
 ds={l:json.loads((BASE/f'{l}.json').read_text()) for l in LANGS};source=ds['de'];errors=[];allowed=[]
 for l,d in ds.items():
  if set(d)!=set(source):errors.append({'language':l,'kind':'keys','missing':sorted(set(source)-set(d)),'extra':sorted(set(d)-set(source))})
  for k,v in d.items():
   if not isinstance(v,str) or not v.strip():errors.append({'language':l,'kind':'empty','key':k});continue
   if sorted(re.findall(r'\{[^{}]+\}',source.get(k,'')))!=sorted(re.findall(r'\{[^{}]+\}',v)):errors.append({'language':l,'kind':'placeholders','key':k})
   if l!='de' and v==source.get(k):
    if v in ALLOW:allowed.append({'language':l,'key':k,'reason':ALLOW[v]})
    else:errors.append({'language':l,'kind':'identical','key':k,'value':v})
 return {'languages':LANGS,'keys':len(source),'errors':errors,'allowedIdentical':allowed,'allowlist':ALLOW}
if __name__=='__main__':
 result=audit();out=ROOT/os.environ.get('E2E_OUTPUT','docs/tests/034')/'i18n-dictionaries.json';out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n')
 print(f"034 dictionaries: {result['keys']} keys × 7 languages, {len(result['errors'])} errors, {len(result['allowedIdentical'])} documented identical values")
 for e in result['errors']:print(json.dumps(e,ensure_ascii=False))
 raise SystemExit(bool(result['errors']))
