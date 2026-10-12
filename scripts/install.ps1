# Installs Designer from a GitHub release (Windows).
#
#   irm https://raw.githubusercontent.com/vinaayakh/designer/main/scripts/install.ps1 | iex
#
# Environment:
#   VERSION        version to install, e.g. 0.2.0 (default: the latest release)
#   INSTALL_DIR    where designer.exe goes (default: %LOCALAPPDATA%\Programs\Designer)
#   DESIGNER_REPO  owner/repo to download from (default: vinaayakh/designer)
#   DESIGNER_DOWNLOAD_URL  a mirror holding the release files (needs VERSION)
#   NO_MODIFY_PATH  set to 1 to leave your PATH alone
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$repo = if ($env:DESIGNER_REPO) { $env:DESIGNER_REPO } else { 'vinaayakh/designer' }
$dir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\Designer' }
$version = if ($env:VERSION) { $env:VERSION } else { 'latest' }

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
  'AMD64' { 'amd64' }
  'ARM64' { 'arm64' }
  default { throw "Unsupported architecture $($env:PROCESSOR_ARCHITECTURE)" }
}

if ($version -eq 'latest') {
  $version = (Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest").tag_name
}
$version = $version.TrimStart('v')

$name = "designer_${version}_windows_$arch"
$base = if ($env:DESIGNER_DOWNLOAD_URL) { $env:DESIGNER_DOWNLOAD_URL } else { "https://github.com/$repo/releases/download/v$version" }
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("designer-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
  Write-Host " Downloading Designer $version (windows/$arch)"
  Invoke-WebRequest "$base/$name.zip" -OutFile "$tmp\$name.zip"
  Invoke-WebRequest "$base/checksums.txt" -OutFile "$tmp\checksums.txt"

  $line = Get-Content "$tmp\checksums.txt" | Where-Object { $_ -match "\s\*?$([regex]::Escape("$name.zip"))$" } | Select-Object -First 1
  if (-not $line) { throw "$name.zip is not in checksums.txt" }
  $want = ($line -split '\s+')[0].ToLower()
  $have = (Get-FileHash "$tmp\$name.zip" -Algorithm SHA256).Hash.ToLower()
  if ($want -ne $have) { throw "Checksum mismatch for $name.zip" }
  Write-Host " Checksum verified"

  Expand-Archive "$tmp\$name.zip" -DestinationPath $tmp -Force
  New-Item -ItemType Directory -Path $dir -Force | Out-Null
  Copy-Item "$tmp\$name\*" $dir -Recurse -Force
  Write-Host " Installed $dir\designer.exe"
} finally {
  Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (-not $env:NO_MODIFY_PATH -and -not (($userPath -split ';') -contains $dir)) {
  [Environment]::SetEnvironmentVariable('Path', ($userPath.TrimEnd(';') + ";$dir"), 'User')
  Write-Host " Added $dir to your PATH (open a new terminal to use it)"
}
if (-not (Get-Command claude -ErrorAction SilentlyContinue)) {
  Write-Host " Designer runs its AI work in Claude Code. Install it and log in: https://claude.com/claude-code"
}
Write-Host " Start it with: designer $HOME\my-system"
