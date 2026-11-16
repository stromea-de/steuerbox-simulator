"""Offline dictionary coverage, no translation service or third party package."""
import json
import re
from html.parser import HTMLParser
from pathlib import Path
import unittest
import importlib.util
ROOT=Path(__file__).resolve().parent.parent
class Dictionaries(unittest.TestCase):
 def test_preparation_copy033b(self):
  base=ROOT/'internal/webui/assets'
  for lang in ['de','en','pl','ru','uk','tr','ro']:
   data=json.loads((base/'i18n'/f'{lang}.json').read_text())
   # 036: setting values is a separate card, not the preparation step title.
   self.assertNotEqual(data['Prüfung einstellen'],data['Prüfung vorbereiten'],lang)
   self.assertTrue(data['Gerätetyp nicht angegeben'],lang)
   self.assertFalse(any('ADR 027' in value for value in data.values()),lang)
  self.assertNotIn('id="manual-cancel"',(base/'index.html').read_text())

 def test_coverage(self):
  base=ROOT/'internal/webui/assets/i18n'
  de=json.loads((base/'de.json').read_text())
  self.assertGreater(len(de),150)
  for lang in ['de','en','pl','ru','uk','tr','ro']:
   data=json.loads((base/(lang+'.json')).read_text())
   self.assertEqual(set(de),set(data),lang)
   self.assertTrue(all(isinstance(v,str) and v.strip() for v in data.values()),lang)
   if lang!='de': self.assertGreater(sum(data[k]!=de[k] for k in de),len(de)*.9,lang)
 def test_static_ui_copy_has_dictionary_keys(self):
  base=ROOT/'internal/webui/assets'
  de=json.loads((base/'i18n/de.json').read_text())
  copy=[]
  class Text(HTMLParser):
   def handle_data(self,value):
    value=value.strip()
    if value and '{{' not in value and re.search('Prüf|Gerät|Einspeis|Bezug|Übung|Zurück|Messwert|Freigabe|Notbetrieb|[äöüß]',value):copy.append(value)
   def handle_starttag(self,tag,attrs):
    for key,value in attrs:
     if key in ['aria-label','data-tip','title','placeholder'] and value and re.search('Prüf|Gerät|[äöüß]',value):copy.append(value)
  for name in ['index.html','protokoll.html']:Text().feed((base/name).read_text())
  # The application mounts the same result markup in both live and saved views.
  for name in ['result.js','guide.js']:
   for template in re.findall(r'innerHTML=`(.*?)`', (base/name).read_text(), re.S):Text().feed(template)
  for value in copy:self.assertIn(value,de)
 def test_embedded_bundle_matches_dictionaries(self):
  spec=importlib.util.spec_from_file_location('bundle_i18n',ROOT/'scripts/build-i18n.py')
  module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
  self.assertEqual((ROOT/'internal/webui/assets/i18n-data.js').read_text(),module.bundle())
if __name__=='__main__' :unittest.main()
