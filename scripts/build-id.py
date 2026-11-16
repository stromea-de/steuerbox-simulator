#!/usr/bin/env python3
"""Identify a clean commit or a deterministic local test snapshot. No Git writes."""
import hashlib
import subprocess
from pathlib import Path


def build_id(root):
    def git(*args):
        return subprocess.run(['git', *args], cwd=root, check=True, stdout=subprocess.PIPE).stdout

    commit = git('rev-parse', '--short', 'HEAD').decode().strip()
    if not git('status', '--porcelain', '--untracked-files=normal').strip():
        return commit
    digest = hashlib.sha256()
    for name in sorted(set(git('ls-files', '--cached', '--others', '--exclude-standard', '-z').split(b'\0')) - {b''}):
        relative = name.decode('utf-8', errors='surrogateescape')
        # Hash build inputs, excluding reports that may quote the resulting ID.
        if not (relative.startswith(('cmd/', 'internal/', 'scripts/', 'packaging/')) or relative in ('go.mod', 'go.sum', '.goreleaser.yaml')):
            continue
        path = root / relative
        digest.update(name + b'\0')
        if path.is_symlink():
            data = b'link\0' + str(path.readlink()).encode()
        elif path.is_file():
            data = b'file\0' + path.read_bytes()
        else:
            data = b'deleted\0'
        digest.update(len(data).to_bytes(8, 'big') + data)
    return commit + '-test.' + digest.hexdigest()[:12] + '-dirty'


if __name__ == '__main__':
    print(build_id(Path(__file__).resolve().parent.parent))
