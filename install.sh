#!/bin/sh
set -eu

version=${LETSGEN_VERSION:-v0.1.0}
install_dir=${LETSGEN_INSTALL_DIR:-"$HOME/.local/bin"}
case "$version" in v[0-9]*.[0-9]*.[0-9]*) ;; *) echo 'Invalid LETSGEN_VERSION' >&2; exit 1 ;; esac
case "$version" in *[!a-zA-Z0-9.-]*) echo 'Invalid LETSGEN_VERSION' >&2; exit 1 ;; esac
case "$(uname -s)" in Darwin) os=darwin ;; Linux) os=linux ;; *) echo 'Use install.ps1 on Windows.' >&2; exit 1 ;; esac
case "$(uname -m)" in arm64|aarch64) arch=arm64 ;; x86_64|amd64) arch=amd64 ;; *) echo 'Unsupported CPU architecture' >&2; exit 1 ;; esac
archive="letsgen_${version}_${os}_${arch}.tar.gz"
base="https://github.com/LetsGenLab/letsgen-cli/releases/download/$version"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
curl --proto '=https' --tlsv1.2 --fail --show-error --silent --location "$base/$archive" -o "$tmp/$archive"
curl --proto '=https' --tlsv1.2 --fail --show-error --silent --location "$base/checksums.txt" -o "$tmp/checksums.txt"
expected=$(awk -v file="$archive" '$2 == file { print $1 }' "$tmp/checksums.txt")
case "$expected" in ''|*[!a-f0-9]*) echo 'Missing or invalid release checksum' >&2; exit 1 ;; esac
[ "${#expected}" -eq 64 ] || { echo 'Invalid checksum length' >&2; exit 1; }
if command -v sha256sum >/dev/null 2>&1; then actual=$(sha256sum "$tmp/$archive" | awk '{print $1}'); else actual=$(shasum -a 256 "$tmp/$archive" | awk '{print $1}'); fi
[ "$actual" = "$expected" ] || { echo 'Checksum mismatch' >&2; exit 1; }
[ "$(tar -tzf "$tmp/$archive")" = "$(printf 'letsgen\nLICENSE')" ] || { echo 'Unexpected archive contents' >&2; exit 1; }
tar -xzf "$tmp/$archive" -C "$tmp"
[ -f "$tmp/letsgen" ] && [ ! -L "$tmp/letsgen" ] || { echo 'Invalid executable' >&2; exit 1; }
[ -f "$tmp/LICENSE" ] && [ ! -L "$tmp/LICENSE" ] || { echo 'Invalid license' >&2; exit 1; }
mkdir -p "$install_dir"
install -m 755 "$tmp/letsgen" "$install_dir/letsgen"
install -m 644 "$tmp/LICENSE" "$install_dir/letsgen.LICENSE"
echo "Installed $version to $install_dir/letsgen"
echo "Add $install_dir to PATH, then run: letsgen auth login"
