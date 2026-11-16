"""Build IDs distinguish saved revisions and deterministic local test snapshots."""
import importlib.util
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

ROOT = Path(__file__).resolve().parent.parent

class BuildID(unittest.TestCase):
    def test_clean_tracked_untracked_deleted_and_ignored(self):
        spec = importlib.util.spec_from_file_location('build_id', ROOT/'scripts/build-id.py')
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root/'internal').mkdir()
            (root/'internal/app.js').write_text('saved')
            status = ''
            paths = b'internal/app.js\0'
            def git(args, **kwargs):
                output = {'rev-parse': b'cafe123\n', 'status': status.encode(), 'ls-files': paths}[args[1]]
                return subprocess.CompletedProcess(args, 0, stdout=output)
            with patch.object(module.subprocess, 'run', side_effect=git):
                self.assertEqual(module.build_id(root), 'cafe123')
                status = ' M app.js\n'
                first = module.build_id(root)
                self.assertRegex(first, r'^cafe123-test\.[0-9a-f]{12}-dirty$')
                self.assertEqual(module.build_id(root), first)
                (root/'internal/app.js').write_text('changed')
                self.assertNotEqual(module.build_id(root), first)
                status = '?? new.js\n'
                paths += b'internal/new.js\0'
                (root/'internal/new.js').write_text('new source')
                untracked = module.build_id(root)
                (root/'internal/new.js').write_text('new source edited')
                self.assertNotEqual(module.build_id(root), untracked)
                (root/'internal/app.js').unlink()
                self.assertRegex(module.build_id(root), r'-dirty$')
                # git omits ignored build outputs; they must not alter the fingerprint.
                stable = module.build_id(root)
                (root/'ignored.exe').write_bytes(b'ignored output')
                paths += b'docs/report.md\0'
                (root/'docs').mkdir()
                (root/'docs/report.md').write_text('Build report with fingerprint')
                self.assertEqual(module.build_id(root), stable)

if __name__ == '__main__':
    unittest.main()
