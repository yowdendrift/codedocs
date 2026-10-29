# Runs codedocs.exe on Windows from PowerShell, for when bash isn't available (Claude Code
# without Git Bash, or other agents). The same as the bash launcher next to it: on first run it
# downloads codedocs.exe from the GitHub release matching ..\VERSION and checks it against the
# release's SHA256SUMS.
#   & "<skill>\bin\codedocs.ps1" status
$ErrorActionPreference = 'Stop'
$d = $PSScriptRoot
$bin = Join-Path $d 'codedocs.exe'
$asset = 'codedocs-windows-amd64.exe'

if (-not (Test-Path $bin)) {
  $ver = "$(Get-Content (Join-Path $d '..\VERSION') -Raw -ErrorAction SilentlyContinue)".Trim()
  $repo = "$(Get-Content (Join-Path $d '..\REPO') -Raw -ErrorAction SilentlyContinue)".Trim()
  if (-not $ver -or -not $repo) { [Console]::Error.WriteLine("codedocs: no binary in $d, and VERSION or REPO is missing so it can't be downloaded."); exit 2 }
  $root = if ($env:CODEDOCS_DOWNLOAD_BASE) { $env:CODEDOCS_DOWNLOAD_BASE } else { "https://github.com/$repo/releases/download" }
  $base = "$root/v$ver"
  $tmp = "$bin.download"
  [Console]::Error.WriteLine("codedocs: first run, downloading $asset v$ver from $base ...")
  $ProgressPreference = 'SilentlyContinue'
  try {
    Invoke-WebRequest -UseBasicParsing -Uri "$base/$asset" -OutFile $tmp
    $sums = (Invoke-WebRequest -UseBasicParsing -Uri "$base/SHA256SUMS").Content
    if ($sums -is [byte[]]) { $sums = [Text.Encoding]::UTF8.GetString($sums) }
  } catch {
    Remove-Item $tmp -ErrorAction SilentlyContinue
    [Console]::Error.WriteLine("codedocs: download failed from $base ($($_.Exception.Message)).")
    [Console]::Error.WriteLine("  If v$ver was released in the last few minutes, the release may still be building; try again shortly.")
    exit 2
  }
  $want = ($sums -split "`n" | ForEach-Object { $p = $_.Trim() -split '\s+'; if ($p.Count -ge 2 -and $p[1].TrimStart('*') -eq $asset) { $p[0] } }) | Select-Object -First 1
  $got = (Get-FileHash -Algorithm SHA256 $tmp).Hash.ToLower()
  if (-not $want -or $got -ne $want.ToLower()) {
    Remove-Item $tmp -ErrorAction SilentlyContinue
    [Console]::Error.WriteLine("codedocs: checksum mismatch for $asset; not using it.")
    exit 2
  }
  Move-Item -Force $tmp $bin
  Unblock-File $bin -ErrorAction SilentlyContinue
  [Console]::Error.WriteLine("codedocs: installed $bin")
}
& $bin @args
exit $LASTEXITCODE
