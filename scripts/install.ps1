<#
.SYNOPSIS
  Install a prebuilt chat-preview binary (Windows x64 / arm64).

  Windows builds are compiled and cross-built in CI but have not been verified
  on a real Windows machine yet (docs/ROADMAP.md, M3). Reports are welcome.

.EXAMPLE
  irm https://raw.githubusercontent.com/gridhra/chat-tool-ui-preview/main/scripts/install.ps1 | iex

.PARAMETER Version
  Version to install, e.g. 0.1.0. Defaults to the latest release.

.PARAMETER InstallDir
  Install directory. Defaults to $env:LOCALAPPDATA\Programs\chat-preview. Never needs elevation.
#>
param(
  [string]$Version = $env:CHAT_PREVIEW_VERSION,
  [string]$InstallDir = $(if ($env:CHAT_PREVIEW_INSTALL_DIR) { $env:CHAT_PREVIEW_INSTALL_DIR } else { "$env:LOCALAPPDATA\Programs\chat-preview" })
)

$ErrorActionPreference = "Stop"
# Windows PowerShell 5.1 does not always default to TLS 1.2.
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$Repo = "gridhra/chat-tool-ui-preview"

$arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
switch ($arch) {
  "X64" { $goarch = "amd64" }
  "Arm64" { $goarch = "arm64" }
  default { throw "no prebuilt Windows binary for $arch. With a Go toolchain: go install github.com/$Repo/cmd/chat-preview@latest" }
}

if (-not $Version) {
  $Version = (Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest").tag_name
}
if (-not $Version) { throw "could not determine the latest release of $Repo; set CHAT_PREVIEW_VERSION" }
$Version = $Version.TrimStart('v')
if ($Version -notmatch '^[0-9]+\.[0-9]+\.[0-9]+$') { throw "invalid version: $Version (expected e.g. 0.1.0)" }

$archive = "chat-preview_${Version}_windows_${goarch}.zip"
$base = "https://github.com/$Repo/releases/download/v$Version"
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $tmp -Force | Out-Null

try {
  Write-Host "Downloading chat-preview $Version (windows/$goarch)"
  $zip = Join-Path $tmp $archive
  Invoke-WebRequest -Uri "$base/$archive" -OutFile $zip -UseBasicParsing

  # Verification is mandatory: failing to fetch or match checksums.txt aborts the install.
  $sums = (Invoke-WebRequest -Uri "$base/checksums.txt" -UseBasicParsing).Content
  if ($sums -is [byte[]]) { $sums = [System.Text.Encoding]::UTF8.GetString($sums) }
  $pattern = '^[0-9a-fA-F]{64}\s+\*?' + [regex]::Escape($archive) + '\s*$'
  $line = ($sums -split "`n") | Where-Object { $_ -match $pattern } | Select-Object -First 1
  if (-not $line) { throw "$archive is not listed in checksums.txt" }
  $expected = ($line -split '\s+')[0].ToLower()
  $actual = (Get-FileHash -Algorithm SHA256 $zip).Hash.ToLower()
  if ($expected -ne $actual) { throw "checksum mismatch for $archive (expected $expected, got $actual); nothing was installed" }
  Write-Host "Checksum OK"

  Expand-Archive -Path $zip -DestinationPath $tmp -Force
  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
  # Stage next to the destination, then replace: an interrupted install never
  # leaves a half-written binary where the old one was.
  $dest = Join-Path $InstallDir "chat-preview.exe"
  $staged = Join-Path $InstallDir ".chat-preview.$PID.exe"
  Copy-Item -Path (Join-Path $tmp "chat-preview.exe") -Destination $staged -Force
  Move-Item -Path $staged -Destination $dest -Force

  Write-Host "Installed $dest"
  & $dest --version
  if (($env:PATH -split ';') -notcontains $InstallDir) {
    Write-Host ""
    Write-Host "Note: $InstallDir is not on your PATH. Hooks and MCP client configs call chat-preview by name, so add it:"
    Write-Host "  [Environment]::SetEnvironmentVariable('PATH', `"$InstallDir;`" + [Environment]::GetEnvironmentVariable('PATH', 'User'), 'User')"
  }
} finally {
  Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
