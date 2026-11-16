#!/usr/bin/env python3
"""Offline install metadata, checksums and local signing key validation. Stdlib only."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import stat


def sha256(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def validate_key(path, root):
    path, root = Path(path).resolve(strict=True), Path(root).resolve()
    if path.is_relative_to(root) or not path.is_file():
        raise ValueError("Signaturschlüssel muss außerhalb des Repositorys liegen")
    if os.name != "nt" and stat.S_IMODE(path.stat().st_mode) & 0o077:
        raise ValueError("Signaturschlüssel benötigt Modus 0600 oder strenger")
    return path


def write_json(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def metadata(dist, version, repository):
    if not re.fullmatch(r"\d+\.\d+\.\d+(?:-[a-zA-Z0-9.-]+)?", version):
        raise ValueError("Ungültige Releaseversion")
    if not re.fullmatch(r"https://github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repository):
        raise ValueError("GitHub-Repository als HTTPS-URL ohne .git/Slash angeben")
    base = f"{repository}/releases/download/v{version}"
    source = dist / f"stromea-steuerbox-simulator_{version}_source.tar.gz"
    source_hash = sha256(source)
    windows = {arch: dist / f"stromea-steuerbox-simulator_{version}_windows_{arch}.zip" for arch in ["amd64", "arm64"]}
    hashes = {arch: sha256(path) for arch, path in windows.items()}
    target = dist / "metadata"
    formula = target / "Formula/stromea-steuerbox-simulator.rb"
    formula.parent.mkdir(parents=True, exist_ok=True)
    formula.write_text(f'''# Maintainer: Stromea <kontakt@stromea.de>
class StromeaSteuerboxSimulator < Formula
  desc "Stromea Steuerbox-Simulator – Software-Simulation per EEBUS"
  homepage "{repository}"
  url "{base}/{source.name}"
  version "{version}"
  sha256 "{source_hash}"
  license "AGPL-3.0-only"
  depends_on "go" => :build

  def install
    system "go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w -buildid= -X github.com/stromea-de/steuerbox-simulator/internal/report.Version={version}", "-o", bin/"stromea-steuerbox-simulator", "./cmd/steuerbox-simulator"
    doc.install "docs/LIZENZEN.md", "docs/third-party"
    prefix.install "LICENSE"
  end

  test do
    assert_match "Stromea Steuerbox-Simulator", shell_output("#{{bin}}/stromea-steuerbox-simulator --help 2>&1")
  end
end
''', encoding="utf-8")
    scoop = {"version": version, "description": "Stromea Steuerbox-Simulator", "homepage": repository,
             "license": "AGPL-3.0-only", "architecture": {}, "bin": "stromea-steuerbox-simulator.exe"}
    for scoop_arch, arch in [("64bit", "amd64"), ("arm64", "arm64")]:
        scoop["architecture"][scoop_arch] = {"url": f"{base}/{windows[arch].name}", "hash": hashes[arch]}
    write_json(target / "bucket/stromea-steuerbox-simulator.json", scoop)
    common = {"PackageIdentifier": "Stromea.SteuerboxSimulator", "PackageVersion": version, "ManifestVersion": "1.9.0"}
    write_json(target / "winget/Stromea.SteuerboxSimulator.yaml", dict(common, DefaultLocale="de-DE", ManifestType="version"))
    write_json(target / "winget/Stromea.SteuerboxSimulator.locale.de-DE.yaml", dict(common,
               PackageLocale="de-DE", Publisher="Stromea", Author="Stromea", PublisherSupportUrl="https://stromea.de/simulator", Description="Stromea Steuerbox-Simulator. Kontakt: kontakt@stromea.de", PackageName="Stromea Steuerbox-Simulator",
               License="AGPL-3.0-only", LicenseUrl=repository + "/blob/v" + version + "/LICENSE",
               ShortDescription="Software-Simulation der Steuerbox per EEBUS mit PDF-Prüfprotokoll. Keine Steuerbox-Hardware.",
               ManifestType="defaultLocale"))
    write_json(target / "winget/Stromea.SteuerboxSimulator.installer.yaml", dict(common,
               InstallerType="zip", NestedInstallerType="portable", Commands=["stromea-steuerbox-simulator"],
               NestedInstallerFiles=[{"RelativeFilePath": "stromea-steuerbox-simulator.exe", "PortableCommandAlias": "stromea-steuerbox-simulator"}],
               Installers=[{"Architecture": wa, "InstallerUrl": f"{base}/{windows[arch].name}",
                            "InstallerSha256": hashes[arch].upper()} for wa, arch in [("x64", "amd64"), ("arm64", "arm64")]],
               ManifestType="installer"))


def checksums(dist):
    # Only distributable artifacts; do not include build working directories or GoReleaser internals.
    files = [p for p in dist.iterdir() if p.is_file() and p.name.endswith((".zip", ".tar.gz", ".deb", ".rpm", ".AppImage"))]
    files += [p for p in (dist / "metadata").rglob("*") if p.is_file()]
    lines = [f"{sha256(p)}  {p.relative_to(dist).as_posix()}\n" for p in sorted(files)]
    (dist / "SHA256SUMS").write_text("".join(lines), encoding="utf-8")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    key = sub.add_parser("key")
    key.add_argument("path", type=Path)
    key.add_argument("root", type=Path)
    manifest = sub.add_parser("metadata")
    manifest.add_argument("dist", type=Path)
    manifest.add_argument("version")
    manifest.add_argument("repository")
    sums = sub.add_parser("checksums")
    sums.add_argument("dist", type=Path)
    args = parser.parse_args()
    if args.command == "key":
        print(validate_key(args.path, args.root))
    elif args.command == "metadata":
        metadata(args.dist, args.version, args.repository)
    else:
        checksums(args.dist)
