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
Push-Location $project
try { go build -o $output .; if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE" } } finally { Pop-Location; Remove-Item Env:GOOS -ErrorAction SilentlyContinue; Remove-Item Env:GOARCH -ErrorAction SilentlyContinue }
if ($Target -eq 'wasm') {
    $web = Join-Path $bin 'web'
    New-Item -ItemType Directory -Force -Path $web | Out-Null
    Copy-Item -LiteralPath $output -Destination (Join-Path $web 'aoz.wasm') -Force
    Copy-Item -LiteralPath (Join-Path $project 'web\index.html') -Destination (Join-Path $web 'index.html') -Force
    $wasmExec = Join-Path (go env GOROOT) 'lib\wasm\wasm_exec.js'
    if (-not (Test-Path -LiteralPath $wasmExec)) { $wasmExec = Join-Path (go env GOROOT) 'misc\wasm\wasm_exec.js' }
    if (-not (Test-Path -LiteralPath $wasmExec)) { throw "wasm_exec.js not found below GoROOT" }
    Copy-Item -LiteralPath $wasmExec -Destination (Join-Path $web 'wasm_exec.js') -Force
    $cache = Join-Path $bin 'data-cache'
    if (Test-Path -LiteralPath $cache) { Copy-Item -LiteralPath $cache -Destination (Join-Path $web 'data') -Recurse -Force }
}
Write-Output $output
