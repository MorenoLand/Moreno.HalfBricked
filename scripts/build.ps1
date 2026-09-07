param([ValidateSet('windows','linux','darwin','wasm')][string]$Target = 'windows')
$ErrorActionPreference = 'Stop'
$project = Split-Path -Parent $PSScriptRoot
$bin = Join-Path $project 'bin'
New-Item -ItemType Directory -Force -Path $bin | Out-Null
switch ($Target) {
    'windows' { $env:GOOS = 'windows'; $env:GOARCH = 'amd64'; $output = Join-Path $bin 'aoz.exe' }
    'linux' { $env:GOOS = 'linux'; $env:GOARCH = 'amd64'; $output = Join-Path $bin 'aoz-linux' }
    'darwin' { $env:GOOS = 'darwin'; $env:GOARCH = 'amd64'; $output = Join-Path $bin 'aoz-darwin' }
    'wasm' { $env:GOOS = 'js'; $env:GOARCH = 'wasm'; $output = Join-Path $bin 'aoz.wasm' }
}
Push-Location (Join-Path $project 'frontend')
try { go build -o $output .; if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE" } } finally { Pop-Location; Remove-Item Env:GOOS -ErrorAction SilentlyContinue; Remove-Item Env:GOARCH -ErrorAction SilentlyContinue }
Write-Output $output
