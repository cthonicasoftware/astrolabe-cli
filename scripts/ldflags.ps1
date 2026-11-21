# Prints the -ldflags string that stamps version metadata into the binary.
# Windows twin of scripts/ldflags.sh; keep the two in sync. Used by the mise
# build task on Windows, where `bash` may resolve to WSL and build for Linux.
#
# Version resolution:
#   1. $env:ASTROLABE_VERSION, if set
#   2. `git describe --tags --always --dirty`
#   3. "dev"
# A leading "v" is stripped so the value is plain SemVer (1.2.3).

$ErrorActionPreference = 'Stop'

$Pkg = 'github.com/cthonicasoftware/astrolabe-cli/internal/version'

$Version = $env:ASTROLABE_VERSION
if (-not $Version) {
    $Version = git describe --tags --always --dirty 2>$null
    if ($LASTEXITCODE -ne 0 -or -not $Version) { $Version = 'dev' }
}
$Version = $Version -replace '^v', ''

$Commit = git rev-parse --short HEAD 2>$null
if ($LASTEXITCODE -ne 0 -or -not $Commit) { $Commit = 'unknown' }

$BuildDate = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")

"-s -w -X $Pkg.Version=$Version -X $Pkg.Commit=$Commit -X $Pkg.BuildDate=$BuildDate"
