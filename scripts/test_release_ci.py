"""Offline release boundary regressions; no publishing or device access."""
import importlib.util
import os
import textwrap
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent.parent

class ReleaseTests(unittest.TestCase):
    @unittest.skipUnless((ROOT / "scripts/export_public.py").exists(), "private exporter")
    def test_export_contains_workflow_and_helpers(self):
        spec = importlib.util.spec_from_file_location('exporter', ROOT / 'scripts/export_public.py')
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        names = {name for _, name in module.selected_files(ROOT)}
        for name in ['.github/workflows/release.yml', 'scripts/release-ci.sh',
                     'scripts/release-assets.py', 'scripts/install-goreleaser.sh',
                     'docs/RELEASE_NOTES_1.0.0.md']:
            self.assertIn(name, names)

    def test_exact_assets_and_checksums(self):
        spec = importlib.util.spec_from_file_location('assets', ROOT / 'scripts/release-assets.py')
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        with tempfile.TemporaryDirectory() as tmp:
            dist = Path(tmp)
            for name in module.asset_names('1.0.0'):
                (dist / name).write_bytes(b'fixture')
            (dist / 'metadata').mkdir()
            (dist / 'metadata/local.json').write_text('local only')
            module.checksums(dist, '1.0.0')
            lines = (dist / 'SHA256SUMS').read_text().splitlines()
            self.assertEqual(len(lines), 11)
            self.assertNotIn('metadata', '\n'.join(lines))
            module.verify(dist, '1.0.0')
            (dist / module.asset_names('1.0.0')[0]).write_bytes(b'corrupt')
            with self.assertRaises(ValueError):
                module.verify(dist, '1.0.0')
            with self.assertRaises(ValueError):
                module.asset_names('1.0.0; echo unsafe')

    def test_workflow_manifest_failures_stop_staging_and_release(self):
        workflow = ROOT / 'public/.github/workflows/release.yml'
        if not workflow.exists():
            workflow = ROOT / '.github/workflows/release.yml'
        source = workflow.read_text()
        for step in ['Stage exact release assets', 'Create stable release after all checks']:
            section = source.split('      - name: ' + step + '\n', 1)[1]
            block = section.split('        run: |\n', 1)[1].split('\n      - ', 1)[0]
            block = textwrap.dedent(block)
            for count, exit_code in [(0, 7), (5, 7), (0, 0), (11, 0), (13, 0), (12, 0)]:
                with self.subTest(step=step, count=count, exit=exit_code), tempfile.TemporaryDirectory() as tmp:
                    root = Path(tmp)
                    (root / 'bin').mkdir()
                    files = []
                    for i in range(count):
                        asset = root / ('asset-' + str(i))
                        asset.write_text('fixture')
                        files.append(str(asset))
                    # Replace only the manifest producer. Neither Python's real
                    # helper nor any GitHub executable is invoked by this shell.
                    producer = root / 'bin/python3'
                    producer.write_text('#!/bin/sh\n' + ''.join(
                        "printf '%s\\n' '" + name + "'\n" for name in files) +
                        'exit ' + str(exit_code) + '\n')
                    producer.chmod(0o755)
                    marker = root / 'release-called'
                    prelude = 'gh() { printf \'%s\\n\' "$@" > "$RELEASE_MARKER"; }\n'
                    env = dict(os.environ, PATH=str(root / 'bin') + os.pathsep + os.environ['PATH'],
                               RUNNER_TEMP=str(root), VERSION='1.0.0', RELEASE_DRAFT='',
                               RELEASE_MARKER=str(marker))
                    result = subprocess.run(['bash', '--noprofile', '--norc', '-e', '-o', 'pipefail',
                                             '-c', prelude + block], cwd=root, env=env,
                                            capture_output=True, text=True)
                    valid = exit_code == 0 and count == 12
                    self.assertEqual(result.returncode == 0, valid, result.stderr)
                    if step.startswith('Create'):
                        self.assertEqual(marker.exists(), valid)
                        if valid:
                            self.assertEqual(marker.read_text().splitlines()[-12:], files)
                    else:
                        staged = root / 'release-assets'
                        self.assertEqual(len(list(staged.glob('*'))), 12 if valid else 0)

    def test_ci_refuses_dirty_or_untagged_checkout(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'scripts').mkdir()
            shutil.copy(ROOT / 'scripts/release-ci.sh', root / 'scripts/release-ci.sh')
            def git(*args):
                subprocess.run(['git', *args], cwd=root, check=True, capture_output=True)
            git('init'); git('config', 'user.name', 'Test'); git('config', 'user.email', 'kontakt@stromea.de')
            git('add', '.'); git('commit', '-m', 'fixture')
            run = lambda: subprocess.run(['bash', 'scripts/release-ci.sh', 'tag'], cwd=root, capture_output=True, text=True)
            result = run()
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('Tag', result.stderr)
            git('tag', 'v1.0.0')
            (root / 'dirty').write_text('fixture')
            result = run()
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('sauberen Checkout', result.stderr)

if __name__ == '__main__':
    unittest.main()
