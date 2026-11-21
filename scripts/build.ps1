# Builds astrolabe.exe with version stamping. Used by `mise run build` on
# Windows; see the note on [tasks.build] in mise.toml.

$ErrorActionPreference = 'Stop'

$ldflags = & "$PSScriptRoot/ldflags.ps1"
go build -ldflags $ldflags -o astrolabe.exe ./cmd/astrolabe
exit $LASTEXITCODE
