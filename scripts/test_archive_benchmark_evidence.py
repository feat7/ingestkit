import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import archive_benchmark_evidence as archive


class ArchiveEvidenceTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name) / "repo"
        self.root.mkdir()
        self.target = Path(self.tmp.name) / "archive"
        self.a = self.root / "benchmarks/results/retired/a.json"
        self.b = self.root / "benchmarks/results/retired/b.json"
        self.a.parent.mkdir(parents=True)
        self.a.write_bytes(b'{"payload":"private-fixture-text"}')
        self.b.write_bytes(b'{"second":true}')
        self.paths = ["benchmarks/results/retired"]

    def test_verified_copy_required_before_remove(self):
        def corrupt(src, dest, **kwargs):
            Path(dest).write_bytes(b"corrupt")
        with patch.object(archive.shutil, "copyfile", side_effect=corrupt):
            with self.assertRaises(ValueError):
                archive.archive_paths(self.root, self.target, self.paths, apply=True)
        self.assertTrue(self.a.exists())
        self.assertTrue(self.b.exists())

    def test_symlink_or_escape_refused(self):
        outside = self.root.parent / "outside"
        outside.write_text("keep")
        (self.a.parent / "link").symlink_to(outside)
        for paths in [self.paths, ["../outside"], [str(outside)], [".git"], [".worktrees"]]:
            with self.subTest(paths=paths), self.assertRaises(ValueError):
                archive.archive_paths(self.root, self.target, paths, apply=True)
        self.assertEqual(outside.read_text(), "keep")

    def test_existing_archive_refused(self):
        self.target.mkdir()
        with self.assertRaises(FileExistsError):
            archive.archive_paths(self.root, self.target, self.paths, apply=True)
        self.assertTrue(self.a.exists())

    def test_archive_control_file_collision_refused(self):
        control = self.root / "manifest.json"
        control.write_text("keep original control bytes")
        with self.assertRaises(ValueError):
            archive.archive_paths(self.root, self.target, ["manifest.json"], apply=True)
        self.assertEqual(control.read_text(), "keep original control bytes")

    def test_late_copy_failure_preserves_all_sources(self):
        real_copy = archive.shutil.copyfile
        def fail_second(src, dest, **kwargs):
            if Path(src).name == "b.json":
                raise OSError("simulated full disk")
            return real_copy(src, dest, **kwargs)
        with patch.object(archive.shutil, "copyfile", side_effect=fail_second):
            with self.assertRaises(OSError):
                archive.archive_paths(self.root, self.target, self.paths, apply=True)
        self.assertTrue(self.a.exists())
        self.assertTrue(self.b.exists())

    def test_only_listed_owned_paths_removed(self):
        keep = self.root / "keep.txt"
        keep.write_text("keep")
        result = archive.archive_paths(self.root, self.target, self.paths, apply=True)
        self.assertFalse(self.a.parent.exists())
        self.assertEqual((self.target / self.a.relative_to(self.root)).read_bytes(), b'{"payload":"private-fixture-text"}')
        self.assertEqual(keep.read_text(), "keep")
        self.assertEqual(len(result.get("files", [])), 2)

    def test_manifest_contains_no_file_contents(self):
        dry = archive.archive_paths(self.root, self.target, self.paths, apply=False)
        self.assertIn("files", dry)
        self.assertFalse(self.target.exists())
        self.assertTrue(self.a.exists())
        result = archive.archive_paths(self.root, self.target, self.paths, apply=True)
        manifest = json.loads((self.target / "manifest.json").read_text())
        self.assertEqual(manifest["files"], result["files"])
        self.assertNotIn("private-fixture-text", json.dumps(manifest))
        self.assertEqual(manifest["checkout_root"], str(self.root.resolve()))
        self.assertEqual(len(manifest["files"][0]["sha256"]), 64)


if __name__ == "__main__":
    unittest.main()
