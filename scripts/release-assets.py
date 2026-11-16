#!/usr/bin/env python3
"""Exact public release manifest, independent of local package-manager drafts."""
import argparse
import hashlib
from pathlib import Path
import re


def asset_names(version):
    if not re.fullmatch(r'\d+\.\d+\.\d+', version):
        raise ValueError('Expected stable semantic version')
    prefix = f'stromea-steuerbox-simulator_{version}'
    names = [f'{prefix}_{os}_{arch}.{ext}' for os, ext in
             [('darwin', 'tar.gz'), ('linux', 'tar.gz'), ('windows', 'zip')]
             for arch in ['amd64', 'arm64']]
    names += [f'{prefix}_linux_{arch}.{ext}' for arch in ['amd64', 'arm64'] for ext in ['deb', 'rpm']]
    return sorted(names + [f'{prefix}_source.tar.gz'])


def checksum_text(dist, version):
    lines = []
    for name in asset_names(version):
        path = dist / name
        if path.is_symlink() or not path.is_file():
            raise ValueError('Missing regular release asset: ' + name)
        lines.append(f'{hashlib.sha256(path.read_bytes()).hexdigest()}  {name}\n')
    return ''.join(lines)


def checksums(dist, version):
    (dist / 'SHA256SUMS').write_text(checksum_text(dist, version))


def verify(dist, version):
    if (dist / 'SHA256SUMS').read_text() != checksum_text(dist, version):
        raise ValueError('Release checksums differ')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('command', choices=['checksums', 'verify', 'list'])
    parser.add_argument('dist', type=Path)
    parser.add_argument('version')
    args = parser.parse_args()
    if args.command == 'checksums':
        checksums(args.dist, args.version)
    else:
        verify(args.dist, args.version)
        if args.command == 'list':
            print('\n'.join(str(args.dist / name) for name in asset_names(args.version) + ['SHA256SUMS']))
