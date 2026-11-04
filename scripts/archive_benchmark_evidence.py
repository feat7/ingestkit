"""Archive selected retired evidence before removing its working-tree files."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess


PROTECTED = {".git", ".aws", ".agents", ".codex", ".worktrees", "node_modules"}


def _hash(path):
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def _inventory(root, paths):
    files, directories, selected = {}, set(), []
    for name in paths:
        rel = Path(name)
        if rel.is_absolute() or not rel.parts or rel == Path("manifest.json") or ".." in rel.parts or any(p in PROTECTED for p in rel.parts):
            raise ValueError("unsafe evidence path")
        source = root / rel
        for parent in [source, *source.parents]:
            if parent == root:
                break
            if parent.is_symlink():
                raise ValueError("symlink evidence is not supported")
        if not source.exists():
            raise FileNotFoundError(str(rel))
        if any(source == prior or source in prior.parents or prior in source.parents for prior in selected):
            raise ValueError("overlapping evidence selections")
        selected.append(source)
        entries = [source, *sorted(source.rglob("*"))] if source.is_dir() else [source]
        for entry in entries:
            if entry.is_symlink() or any(p in PROTECTED for p in entry.relative_to(root).parts):
                raise ValueError("protected path or symlink in evidence")
            if entry.is_dir():
                directories.add(entry)
            elif entry.is_file():
                files[str(entry.relative_to(root))] = {"path": str(entry.relative_to(root)), "size": entry.stat().st_size, "sha256": _hash(entry)}
            else:
                raise ValueError("only regular evidence files are supported")
    return files, directories


def _refuse_archive_symlinks(destination):
    for component in (destination,*destination.parents):
        if component.is_symlink():
            raise ValueError("archive path or ancestor is a symlink")


def archive_paths(repo_root, archive_root, relative_paths, *, apply=False):
    root, destination = Path(repo_root), Path(archive_root)
    if root.is_symlink() or not root.is_dir():
        raise ValueError("checkout root must be a real directory")
    root, destination = root.resolve(), destination.absolute()
    _refuse_archive_symlinks(destination)
    if destination.resolve().is_relative_to(root):
        raise ValueError("archive must be outside the checkout")
    if destination.exists() or destination.is_symlink():
        raise FileExistsError(str(destination))
    files, directories = _inventory(root, relative_paths)
    revision = subprocess.run(["git", "-C", str(root), "rev-parse", "HEAD"], capture_output=True, text=True)
    tracked = subprocess.run(["git", "-C", str(root), "ls-files", "-z"], capture_output=True).stdout.decode().split("\0")
    for name, entry in files.items():
        entry["tracked"] = name in tracked
        entry["reason"] = "retired or superseded evidence; preserve exact bytes before removal"
    manifest = {"schema": 1, "checkout_root": str(root), "source_sha": revision.stdout.strip() if revision.returncode == 0 else None, "paths": relative_paths, "files": list(files.values()), "verified": False, "removed": []}
    if not apply:
        return manifest
    _refuse_archive_symlinks(destination)
    destination.mkdir(parents=True, mode=0o700, exist_ok=False)
    for name in files:
        target = destination / name
        target.parent.mkdir(parents=True, mode=0o700, exist_ok=True)
        shutil.copyfile(root / name, target, follow_symlinks=False)
        target.chmod(0o600)
        with target.open("rb") as handle:
            os.fsync(handle.fileno())
    _refuse_archive_symlinks(destination)
    # The second phase starts only after every destination and every source agrees.
    current, _ = _inventory(root, relative_paths)
    if set(current) != set(files):
        raise ValueError("source inventory changed while archiving")
    for name, entry in files.items():
        if current[name]["sha256"] != entry["sha256"] or _hash(destination / name) != entry["sha256"]:
            raise ValueError("evidence checksum differs; originals preserved")
    _refuse_archive_symlinks(destination)
    manifest["verified"] = True
    record = destination / "manifest.json"
    def save():
        with record.open("w") as handle:
            json.dump(manifest, handle, indent=2)
            handle.write("\n")
            handle.flush()
            os.fsync(handle.fileno())
        record.chmod(0o600)
    save()
    for name in files:
        if _hash(root / name) != files[name]["sha256"]:
            raise ValueError("source changed before removal; verified archive retained")
        (root / name).unlink()
        manifest["removed"].append(name)
    for directory in sorted(directories, key=lambda p: len(p.parts), reverse=True):
        directory.rmdir()  # Never recursively delete an unlisted newly-created file.
    save()
    return manifest


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", type=Path, default=Path.cwd())
    parser.add_argument("--archive", type=Path, required=True)
    parser.add_argument("--apply", action="store_true")
    parser.add_argument("paths", nargs="+")
    args = parser.parse_args()
    result = archive_paths(args.repo, args.archive, args.paths, apply=args.apply)
    print(json.dumps(result, indent=2))
