#!/usr/bin/env python3
"""Exercise the Unix installer offline against locally built release archives."""
import os
from pathlib import Path
import platform
import subprocess
import sys
import tempfile

root = Path(__file__).resolve().parents[1]
version = sys.argv[1] if len(sys.argv) > 1 else "v0.1.0-alpha.1"
os_name = {"Darwin": "darwin", "Linux": "linux"}[platform.system()]
arch = {"arm64": "arm64", "aarch64": "arm64", "x86_64": "amd64", "AMD64": "amd64"}[platform.machine()]
archive = f"letsgen_{version}_{os_name}_{arch}.tar.gz"
assert (root / "dist" / version / archive).exists(), "Build release archives first"
with tempfile.TemporaryDirectory(prefix="letsgen-installer-test-") as work:
    temp = Path(work)
    mock_bin = temp / "mocks"
    mock_bin.mkdir()
    curl = mock_bin / "curl"
    curl.write_text('''#!/usr/bin/env python3
import os,pathlib,shutil,sys
args=sys.argv[1:]
dest=pathlib.Path(args[args.index('-o')+1])
url=next(arg for arg in args if arg.startswith('https://'))
assert url.startswith('https://github.com/LetsGenLab/letsgen-cli/releases/download/'+os.environ['LETSGEN_VERSION']+'/')
name=url.rsplit('/',1)[1]
assert name in [os.environ['TEST_ARCHIVE'],'checksums.txt']
source=pathlib.Path(os.environ['TEST_RELEASE'])/name
shutil.copyfile(source,dest)
if name=='checksums.txt' and os.environ.get('TEST_BAD_CHECKSUM'):
 lines=dest.read_text().splitlines()
 dest.write_text(chr(10).join(('0'*64+'  '+os.environ['TEST_ARCHIVE']) if line.split()[1]==os.environ['TEST_ARCHIVE'] else line for line in lines)+chr(10))
''')
    curl.chmod(0o755)
    env = {**os.environ, "PATH": str(mock_bin) + os.pathsep + os.environ["PATH"], "LETSGEN_VERSION": version,
           "LETSGEN_INSTALL_DIR": str(temp / "installed"), "TEST_ARCHIVE": archive, "TEST_RELEASE": str(root / "dist" / version)}
    subprocess.run(["sh", str(root / "install.sh")], env=env, check=True, capture_output=True, text=True, timeout=30)
    actual = subprocess.check_output([str(temp / "installed" / "letsgen"), "version"], text=True).strip()
    assert actual == version, f"Wrong installed version: {actual}"
    assert (temp / "installed" / "letsgen.LICENSE").read_bytes() == (root / "LICENSE").read_bytes(), "Missing license notice"
    env.update(LETSGEN_INSTALL_DIR=str(temp / "rejected"), TEST_BAD_CHECKSUM="1")
    failed = subprocess.run(["sh", str(root / "install.sh")], env=env, capture_output=True, text=True, timeout=30)
    assert failed.returncode != 0 and not (temp / "rejected" / "letsgen").exists(), "Checksum mismatch was installed"
    print("Unix installer: native binary runs; corrupted checksum rejected; temporary files cleaned")
