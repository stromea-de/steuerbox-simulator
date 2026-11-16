"""Offline regression tests for release integrity and install metadata."""
import hashlib
import importlib.util
import json
import re
import shutil
import subprocess
import tarfile
from pathlib import Path
import tempfile
import unittest

SPEC = importlib.util.spec_from_file_location("distribution", Path(__file__).with_name("distribution.py"))
distribution = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(distribution)


class DistributionTests(unittest.TestCase):
    def test_product_wording_and_packaging_names(self):
        root = Path(__file__).resolve().parent.parent
        files = [root / "README.md", root / ".goreleaser.yaml", root / "docs/VERTEILUNG.md"]
        if (root / "docs/SIGNPATH_ENTWURF.md").exists():
            files.append(root / "docs/SIGNPATH_ENTWURF.md")
        files += list((root / "internal").rglob("*.go"))
        files += list((root / "internal/webui/assets").glob("*.html"))
        files += list((root / "packaging/appimage").glob("*"))
        files += [root / "scripts/distribution.py", root / "scripts/appimage.sh", root / "scripts/release.sh", root / "scripts/check.sh"]
        for path in files:
            if path.name.endswith("_test.go"):
                continue
            text = path.read_text()
            for old in ["Stromea EEBUS Prüftool", "Stromea Prüftool", "stromea-eebus", "Stromea.EEBUSPrueftool", "0100-600"]:
                self.assertNotIn(old, text, str(path))
            # Only the mandated negative trademark wording may contain this
            # word. Other certification/conformity assertions fail the check.
            text = text.replace("herausgegeben oder zertifiziert.", "")
            self.assertIsNone(re.search(r"\b(?:konform|zertifiziert)(?:e|er|es|en|em)?\b", text, re.IGNORECASE), str(path))
        notice = "Die EEBUS-Wort-/Bildmarke ist eine eingetragene Marke der EEBus Initiative e.V. EEBUS bezeichnet hier den unterstützten Kommunikationsstandard. Der Stromea Steuerbox-Simulator ist ein unabhängiges Open-Source-Projekt und wird weder von der EEBus Initiative e.V. noch von der EEBUService GmbH herausgegeben oder zertifiziert. Er ist keine Steuerbox-Hardware und ersetzt keine Konformitätsprüfung."
        for path in [root / "README.md", root / "docs/VERTEILUNG.md"]:
            self.assertIn(notice, path.read_text(), str(path))
        config = (root / ".goreleaser.yaml").read_text()
        for field in ["project_name", "binary", "package_name"]:
            self.assertIn(field + ": stromea-steuerbox-simulator", config)
        self.assertIn("release:\n  disable: true", config)

    def test_default_port_in_cli_and_distribution_docs(self):
        root = Path(__file__).resolve().parent.parent
        self.assertIn('"listen", "127.0.0.1:33287"', (root / "cmd/steuerbox-simulator/main.go").read_text())
        for name in ["README.md", "docs/VERTEILUNG.md"]:
            text = (root / name).read_text()
            self.assertIn("127.0.0.1:33287", text)
            self.assertNotIn(":8080", text)
        for path in (root / "packaging").rglob("*"):
            if path.is_file():
                self.assertNotIn(":8080", path.read_text(), str(path))

    def test_appdir_contains_renamed_executable_and_desktop_entry(self):
        root = Path(__file__).resolve().parent.parent
        slug = "stromea-steuerbox-simulator"
        with tempfile.TemporaryDirectory() as tmp:
            staging = Path(tmp)
            (staging / "scripts").mkdir()
            shutil.copy(root / "scripts/appimage.sh", staging / "scripts/appimage.sh")
            shutil.copytree(root / "packaging", staging / "packaging")
            (staging / "dist").mkdir()
            binary = staging / slug
            binary.write_text("#!/bin/sh\nprintf '%s\\n' \"$@\"\n")
            binary.chmod(0o755)
            for arch in ["amd64", "arm64"]:
                with tarfile.open(staging / f"dist/{slug}_0.2.0_linux_{arch}.tar.gz", "w:gz") as archive:
                    archive.add(binary, arcname=slug)
                subprocess.run(["bash", str(staging / "scripts/appimage.sh"), arch, "0.2.0"], check=True, capture_output=True)
                appdir = staging / f"dist/{slug}-{arch}.AppDir"
                self.assertTrue((appdir / f"usr/bin/{slug}").is_file())
                self.assertTrue((appdir / f"{slug}.svg").is_file())
                desktop = (appdir / f"{slug}.desktop").read_text()
                self.assertIn("Name=Stromea Steuerbox-Simulator", desktop)
                self.assertIn(f"Exec={slug} serve", desktop)
                self.assertIn(f"Icon={slug}", desktop)
                result = subprocess.run([str(appdir / "AppRun"), "serve", "--help"], check=True, capture_output=True, text=True)
                self.assertEqual(result.stdout, "serve\n--help\n")

    def test_metadata_uses_real_artifact_hashes(self):
        with tempfile.TemporaryDirectory() as tmp:
            dist = Path(tmp)
            for suffix in ["source.tar.gz", "windows_amd64.zip", "windows_arm64.zip"]:
                (dist / ("stromea-steuerbox-simulator_0.2.0_" + suffix)).write_bytes(suffix.encode())
            distribution.metadata(dist, "0.2.0", "https://github.com/stromea-de/steuerbox-simulator")
            scoop = json.loads((dist / "metadata/bucket/stromea-steuerbox-simulator.json").read_text())
            for arch, goarch in [("64bit", "amd64"), ("arm64", "arm64")]:
                self.assertEqual(scoop["architecture"][arch]["hash"], hashlib.sha256(("windows_" + goarch + ".zip").encode()).hexdigest())
                self.assertTrue(scoop["architecture"][arch]["url"].endswith("windows_" + goarch + ".zip"))
            formula = (dist / "metadata/Formula/stromea-steuerbox-simulator.rb").read_text()
            self.assertIn('license "AGPL-3.0-only"', formula)
            base = "https://github.com/stromea-de/steuerbox-simulator"
            self.assertIn(base + "/releases/download/v0.2.0/", formula)
            self.assertEqual(scoop["homepage"], base)
            for arch in scoop["architecture"].values():
                self.assertTrue(arch["url"].startswith(base + "/releases/download/v0.2.0/"))
            self.assertIn('system "go", "build"', formula)
            self.assertIn('-X github.com/stromea-de/steuerbox-simulator/internal/report.Version=0.2.0', formula)
            self.assertNotIn("bottle", formula)
            self.assertIn("class StromeaSteuerboxSimulator < Formula", formula)
            self.assertIn('bin/"stromea-steuerbox-simulator"', formula)
            self.assertEqual(scoop["bin"], "stromea-steuerbox-simulator.exe")
            locale = json.loads((dist / "metadata/winget/Stromea.SteuerboxSimulator.locale.de-DE.yaml").read_text())
            self.assertEqual(locale["LicenseUrl"], base + "/blob/v0.2.0/LICENSE")
            self.assertEqual(locale["PackageName"], "Stromea Steuerbox-Simulator")
            self.assertEqual(locale["Publisher"], "Stromea")
            self.assertEqual(locale["PublisherSupportUrl"], "https://stromea.de/simulator")
            self.assertIn("kontakt@stromea.de", locale["Description"])
            self.assertIn("# Maintainer: Stromea <kontakt@stromea.de>", formula)
            self.assertIn("maintainer: Stromea <kontakt@stromea.de>", (Path(__file__).resolve().parent.parent / ".goreleaser.yaml").read_text())
            installer = json.loads((dist / "metadata/winget/Stromea.SteuerboxSimulator.installer.yaml").read_text())
            for entry in installer["Installers"]:
                self.assertTrue(entry["InstallerUrl"].startswith(base + "/releases/download/v0.2.0/"))
            self.assertEqual(installer["Commands"], ["stromea-steuerbox-simulator"])
            self.assertEqual(installer["NestedInstallerFiles"][0]["PortableCommandAlias"], "stromea-steuerbox-simulator")
            self.assertEqual([v["Architecture"] for v in installer["Installers"]], ["x64", "arm64"])
            self.assertEqual(installer["NestedInstallerFiles"][0]["RelativeFilePath"], "stromea-steuerbox-simulator.exe")
            distribution.checksums(dist)
            sums = (dist / "SHA256SUMS").read_text()
            self.assertIn("metadata/Formula/stromea-steuerbox-simulator.rb", sums)
            self.assertNotIn("SHA256SUMS", sums)
            (dist / "SHA256SUMS.minisig").write_bytes(b"signature")
            distribution.checksums(dist)
            self.assertEqual((dist / "SHA256SUMS").read_text(), sums)

    def test_metadata_rejects_missing_artifacts_and_bad_urls(self):
        with tempfile.TemporaryDirectory() as tmp:
            for url in ["https://github.com/o/r;evil", "http://github.com/o/r", "https://example.invalid/o/r"]:
                with self.assertRaises(ValueError):
                    distribution.metadata(Path(tmp), "0.2.0", url)
            with self.assertRaises(ValueError):
                distribution.metadata(Path(tmp), "../../bad", "https://github.com/o/r")
            with self.assertRaises(FileNotFoundError):
                distribution.metadata(Path(tmp), "0.2.0", "https://github.com/o/r")

    def test_secret_must_be_private_and_outside_checkout(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / "repo"
            root.mkdir()
            key = root / "secret.key"
            key.write_text("never a real key")
            key.chmod(0o600)
            with self.assertRaises(ValueError):
                distribution.validate_key(key, root)
            external = Path(tmp) / "external.key"
            external.write_text("never a real key")
            external.chmod(0o644)
            with self.assertRaises(ValueError):
                distribution.validate_key(external, root)
            external.chmod(0o600)
            (root / "link").symlink_to(external)
            self.assertEqual(distribution.validate_key(root / "link", root), external.resolve())
            self.assertEqual(distribution.validate_key(external, root), external.resolve())


if __name__ == "__main__":
    unittest.main()
