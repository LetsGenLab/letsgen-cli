#!/usr/bin/env python3
"""Verify release checksums and safe executable archive contents."""
import hashlib
from pathlib import Path
import sys
import tarfile
import zipfile

root = Path(sys.argv[1])
entries = (root / "checksums.txt").read_text().splitlines()
assert len(entries) == 6, "Expected six platform archives"
seen = set()
for line in entries:
    digest, name = line.split()
    assert Path(name).name == name and name not in seen, "Invalid archive name"
    seen.add(name)
    path = root / name
    assert hashlib.sha256(path.read_bytes()).hexdigest() == digest, f"Checksum failed: {name}"
    if name.endswith(".zip"):
        with zipfile.ZipFile(path) as archive:
            assert archive.namelist() == ["letsgen.exe"], "Unexpected Windows archive content"
    else:
        with tarfile.open(path) as archive:
            entries = archive.getmembers()
            assert len(entries) == 1 and entries[0].name == "letsgen" and entries[0].isfile(), "Unsafe archive content"
    print("Verified", name)
