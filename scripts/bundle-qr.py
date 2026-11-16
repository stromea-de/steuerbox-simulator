#!/usr/bin/env python3
"""Reproduce the offline QR bundle from the pinned npm archive (stdlib only)."""
import argparse
import base64
import hashlib
import json
from pathlib import Path
import tarfile

ROOT = Path(__file__).resolve().parent.parent

def bundle(archive):
    expected = json.loads((ROOT / 'internal/webui/assets/qr-vendor.json').read_text())['integrity']
    actual = 'sha512-' + base64.b64encode(hashlib.sha512(archive.read_bytes()).digest()).decode()
    if expected != actual:
        raise ValueError('QR archive integrity mismatch')
    with tarfile.open(archive) as source:
        license_text = source.extractfile('package/LICENSE-MIT').read().decode()
        parts = ['/* @paulmillr/qr 0.3.0; MIT selected. Bundled without code changes.\n' + license_text + '\n*/\n(() => {\nconst encoder = {};\n(function(exports) {\n']
        parts += [source.extractfile('package/index.js').read().decode(), '\n})(encoder);\nconst decoder = {};\n(function(exports, require) {\n']
        parts += [source.extractfile('package/decode.js').read().decode(), "\n})(decoder, name => { if (name !== './index.js') throw new Error('Unknown QR module'); return encoder; });\nglobalThis.StromeaQR = {encode: encoder.default, decode: decoder.default, Bitmap: encoder.Bitmap};\n})();\n"]
        (ROOT / 'internal/webui/assets/qr-vendor.js').write_text(''.join(parts))
        (ROOT / 'docs/third-party/paulmillr-qr-0.3.0-MIT.txt').write_text(license_text)
        (ROOT / 'docs/third-party/paulmillr-qr-0.3.0-README.txt').write_bytes(source.extractfile('package/README.md').read())

if __name__ == '__main__':
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('archive', type=Path)
    bundle(p.parse_args().archive)
