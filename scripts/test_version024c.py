"""Exercise release version selection with local command doubles; no Git writes."""
import json, os, shutil, subprocess, tempfile, unittest
from pathlib import Path
ROOT=Path(__file__).resolve().parent.parent
class ReleaseVersion(unittest.TestCase):
 def test_tag_and_development_builds(self):
  with tempfile.TemporaryDirectory() as tmp:
   root=Path(tmp);(root/'scripts').mkdir();shutil.copy(ROOT/'scripts/release.sh',root/'scripts/release.sh');bin=root/'tools';bin.mkdir()
   shim='''#!/usr/bin/python3
import os,sys,json
from pathlib import Path
name=Path(sys.argv[0]).name;args=sys.argv[1:]
if name=='uname':print('Darwin')
elif name=='git':
 if args[0]=='describe':
  if not os.environ.get('TEST_TAG'):sys.exit(128)
  print(os.environ['TEST_TAG'])
 elif args[0]=='rev-parse':print('cafe123')
 elif args[0]=='diff':sys.exit(int(os.environ.get('TEST_DIRTY','0')))
 elif args[0]=='show':print('1793440000')
elif name=='python3' and args[0]=='scripts/build-id.py':print('cafe123-test.fixture-dirty' if os.environ.get('TEST_DIRTY')=='1' else 'cafe123')
elif name=='goreleaser':
 if args==['--version']:print('goreleaser version 2.20.0')
 elif args[0]=='release':Path('captured.json').write_text(json.dumps({k:os.environ.get(k) for k in ['STROMEA_REPORT_VERSION','STROMEA_REPORT_COMMIT']}))
'''
   for name in ['git','goreleaser','uname','python3']:(bin/name).write_text(shim);(bin/name).chmod(0o700)
   for tag,dirty,mode,expected in [('',0,'snapshot',('', 'cafe123')),('',1,'snapshot',('', 'cafe123-test.fixture-dirty')),('v1.2.3',0,'tag',('v1.2.3','cafe123'))]:
    env={**os.environ,'PATH':str(bin)+':/usr/bin:/bin','TEST_TAG':tag,'TEST_DIRTY':str(dirty)}
    subprocess.run(['/bin/bash',str(root/'scripts/release.sh'),mode,'--unsigned'],env=env,check=True,stdout=subprocess.DEVNULL)
    data=json.loads((root/'captured.json').read_text());self.assertEqual((data['STROMEA_REPORT_VERSION'],data['STROMEA_REPORT_COMMIT']),expected)
if __name__=='__main__':unittest.main()
