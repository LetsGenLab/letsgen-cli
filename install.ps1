param(
  [string]$Version = 'v0.1.0-alpha.1',
  [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'Programs\LetsGen')
)
$ErrorActionPreference = 'Stop'
if ($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.-]+)?$') { throw 'Invalid version' }
$architecture = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
$arch = switch ($architecture) { 'Arm64' { 'arm64' } 'X64' { 'amd64' } default { throw 'Unsupported architecture' } }
$archive = "letsgen_${Version}_windows_${arch}.zip"
$base = "https://github.com/LetsGenLab/letsgen-cli/releases/download/$Version"
$temp = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $temp | Out-Null
try {
  Invoke-WebRequest -Uri "$base/$archive" -OutFile (Join-Path $temp $archive)
  Invoke-WebRequest -Uri "$base/checksums.txt" -OutFile (Join-Path $temp 'checksums.txt')
  $lines = @(Get-Content (Join-Path $temp 'checksums.txt') | Where-Object { $_ -match "^[a-f0-9]{64}  $([regex]::Escape($archive))$" })
  if ($lines.Count -ne 1) { throw 'Missing or invalid checksum' }
  $expected = $lines[0].Substring(0, 64)
  if ((Get-FileHash (Join-Path $temp $archive) -Algorithm SHA256).Hash.ToLowerInvariant() -ne $expected) { throw 'Checksum mismatch' }
  Add-Type -AssemblyName System.IO.Compression.FileSystem
  $zip = [IO.Compression.ZipFile]::OpenRead((Join-Path $temp $archive))
  try {
    if ($zip.Entries.Count -ne 2 -or $zip.Entries[0].FullName -ne 'letsgen.exe' -or $zip.Entries[1].FullName -ne 'LICENSE') { throw 'Unexpected archive contents' }
  } finally { $zip.Dispose() }
  Expand-Archive -LiteralPath (Join-Path $temp $archive) -DestinationPath (Join-Path $temp 'unpacked')
  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
  Copy-Item -LiteralPath (Join-Path $temp 'unpacked\letsgen.exe') -Destination (Join-Path $InstallDir 'letsgen.exe') -Force
  Copy-Item -LiteralPath (Join-Path $temp 'unpacked\LICENSE') -Destination (Join-Path $InstallDir 'letsgen.LICENSE') -Force
  Write-Output "Installed $Version to $InstallDir. Add it to PATH, then run: letsgen auth login"
} finally { Remove-Item -LiteralPath $temp -Recurse -Force }
