# Installs the /codedocs skill from this folder for Claude Code at user level. For working on
# codedocs itself; everyone else installs the plugin (see README).
#   .\install.ps1         link %USERPROFILE%\.claude\skills\codedocs to .\skills\codedocs (edits here are live)
#   .\install.ps1 -Copy   copy it there instead (a frozen snapshot)
param([switch]$Copy)

$ErrorActionPreference = 'Stop'
$src = Join-Path $PSScriptRoot 'skills\codedocs'
$skillsDir = Join-Path $env:USERPROFILE '.claude\skills'
$dest = Join-Path $skillsDir 'codedocs'
if (-not (Test-Path (Join-Path $src 'SKILL.md'))) { throw "Can't find $src\SKILL.md. Run this from the docs-skill folder." }
$version = (Get-Content (Join-Path $src 'VERSION') -Raw).Trim()

# The skill needs its binary. Build it if Go is available.
$exe = Join-Path $src 'bin\codedocs.exe'
if (-not (Test-Path $exe)) {
  if (Get-Command go -ErrorAction SilentlyContinue) {
    Write-Host 'Building codedocs.exe...'
    & (Join-Path $PSScriptRoot 'build.ps1')
  } else {
    Write-Warning "skills\codedocs\bin\codedocs.exe is missing and Go isn't installed. It will be downloaded from the GitHub release on first use, or install Go (https://go.dev/dl/) and run .\build.ps1."
  }
}

# v0.1.0 installed as 'docs', which clashed with the built-in docs skill. Remove that link.
$old = Join-Path $skillsDir 'docs'
if (Test-Path $old) {
  $o = Get-Item $old -Force
  if ($o.LinkType -and ("$($o.Target)" -eq $src)) { $o.Delete(); Write-Host "Removed old link $old" }
}

New-Item -ItemType Directory -Force -Path $skillsDir | Out-Null
if (Test-Path $dest) {
  $item = Get-Item $dest -Force
  if ($item.LinkType) {
    $item.Delete()   # remove the link itself, never its target
  } else {
    $backup = "$dest.bak-$(Get-Date -Format yyyyMMdd-HHmmss)"
    Rename-Item $dest $backup
    Write-Host "Existing folder moved to $backup"
  }
}

if ($Copy) {
  Copy-Item $src $dest -Recurse
  Write-Host "Copied /codedocs $version to $dest"
} else {
  New-Item -ItemType Junction -Path $dest -Target $src | Out-Null
  Write-Host "Linked $dest -> $src  (/codedocs $version; edits in .\skills\codedocs are live)"
}
if (Test-Path $exe) { Write-Host "Tool: $(& $exe version)" }
Write-Host 'Start a new Claude Code session to pick it up.'
Write-Host 'If the codedocs plugin is also installed, /codedocs runs this copy and /codedocs:codedocs the plugin.'
