#!/usr/bin/env python3
"""Build local release archives. Never tags, pushes or publishes anything."""
import argparse
import gzip
import hashlib
import io
import os
from pathlib import Path
import re
import subprocess
import tarfile
import zipfile

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("version", help="semver tag, e.g. v0.1.0")
parser.add_argument("--target", action="append", choices=["darwin/arm64", "darwin/amd64", "linux/arm64", "linux/amd64", "windows/arm64", "windows/amd64"])
parser.add_argument("--output", default="dist")
args = parser.parse_args()
if not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.-]+)?", args.version):
    parser.error("version must be a semver tag")
root = Path(__file__).resolve().parents[1]
dest = (root / args.output / args.version).resolve()
dest.mkdir(parents=True, exist_ok=True)
sha = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip()
checksums = []
for target in args.target or ["darwin/arm64", "darwin/amd64", "linux/arm64", "linux/amd64", "windows/arm64", "windows/amd64"]:
    goos, arch = target.split("/")
    name = "letsgen.exe" if goos == "windows" else "letsgen"
    binary = dest / name
    env = {**os.environ, "CGO_ENABLED": "0", "GOOS": goos, "GOARCH": arch}
    subprocess.run(["go", "build", "-trimpath", "-ldflags", f"-s -w -X main.version={args.version}", "-o", str(binary), "./cmd/letsgen"], cwd=root, env=env, check=True, timeout=180)
    data = binary.read_bytes()
    archive = dest / f"letsgen_{args.version}_{goos}_{arch}.{'zip' if goos == 'windows' else 'tar.gz'}"
    if goos == "windows":
        with zipfile.ZipFile(archive, "w", compression=zipfile.ZIP_DEFLATED) as z:
            info = zipfile.ZipInfo(name, date_time=(2020, 1, 1, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            z.writestr(info, data)
    else:
        with archive.open("wb") as out, gzip.GzipFile(filename="", mode="wb", fileobj=out, mtime=0) as gz, tarfile.open(fileobj=gz, mode="w") as tar:
            info = tarfile.TarInfo(name)
            info.size, info.mode, info.mtime = len(data), 0o755, 0
            tar.addfile(info, io.BytesIO(data))
    binary.unlink()
    digest = hashlib.sha256(archive.read_bytes()).hexdigest()
    checksums.append(f"{digest}  {archive.name}\n")
    print(f"{target}: {len(data):,} bytes; {archive.name}", flush=True)
(dest / "checksums.txt").write_text("".join(checksums))
(dest / "source.txt").write_text(f"Version: {args.version}\nCommit: {sha}\n")
print(f"Prepared {dest}. Review before tagging or uploading to LetsGenLab.")
