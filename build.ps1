# Builds the codedocs binaries into skills\codedocs\bin. Needs Go 1.22+.
#   .\build.ps1         this machine only (skills\codedocs\bin\codedocs.exe)
#   .\build.ps1 -All    Windows, macOS (Intel + Apple silicon), Linux (x64 + arm64)
param([switch]$All)
$ErrorActionPreference = 'Stop'
$v = (Get-Content (Join-Path $PSScriptRoot 'skills\codedocs\VERSION') -Raw).Trim()
$targets = if ($All) { @('windows/amd64', 'darwin/arm64', 'darwin/amd64', 'linux/amd64', 'linux/arm64') } else { @('windows/amd64') }
# A running viewer locks codedocs.exe on Windows: stop any first.
$exe = Join-Path $PSScriptRoot 'skills\codedocs\bin\codedocs.exe'
if (Test-Path $exe) { & $exe stop --all 2>$null | Out-Null }
Push-Location (Join-Path $PSScriptRoot 'tool')
try {
  foreach ($t in $targets) {
    $os, $arch = $t.Split('/')
    $name = if ($os -eq 'windows') { 'codedocs.exe' } else { "codedocs-$os-$arch" }
    $env:CGO_ENABLED = '0'; $env:GOOS = $os; $env:GOARCH = $arch
    go build -trimpath -ldflags "-s -w -X main.version=$v" -o "..\skills\codedocs\bin\$name" .
    if ($LASTEXITCODE -ne 0) { throw "go build failed for $t" }
    Write-Host "built skills\codedocs\bin\$name ($v)"
  }
} finally {
  Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
  Pop-Location
}
