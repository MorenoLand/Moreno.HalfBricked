# -WebMode picks what the WASM bundle is:
#   hosted (default)  bin/web      the page plus the game's extracted data (data/), for hosting it yourself (your iPad, a server).
#   apk               bin/web-apk  the page with NO game data: each visitor picks their own Age of Zombies APK and the game
#                                  builds itself from it in their browser (the APK never leaves their device).
param([ValidateSet('windows','linux','darwin','wasm','all')][string]$Target = 'all', [ValidateSet('hosted','apk')][string]$WebMode = 'hosted')
$ErrorActionPreference = 'Stop'
if ($Target -eq 'all') {
    & $PSCommandPath -Target windows
    if (-not $?) { throw 'Windows build failed' }
    & $PSCommandPath -Target wasm -WebMode $WebMode
    if (-not $?) { throw 'WASM build failed' }
    return
}
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
$stagedResource = $null
try {
    if ($Target -eq 'windows') {
        $resource = Join-Path $project 'resources\icon_windows_amd64.syso'
        $windres = Get-Command windres -ErrorAction SilentlyContinue
        if ($null -ne $windres) {
            & $windres.Source -i 'resources/icon.rc' -o 'resources/icon_windows_amd64.syso'
            if ($LASTEXITCODE -ne 0) { throw "windres failed with exit code $LASTEXITCODE" }
        }
        if (-not (Test-Path -LiteralPath $resource)) { throw "Windows icon resource not found: $resource" }
        $stagedResource = Join-Path $project 'icon_windows_amd64.syso'
        Copy-Item -LiteralPath $resource -Destination $stagedResource -Force
    }
    # The Windows program is a GUI app: no console window (errors show in a message box, the log goes to a file).
    if ($Target -eq 'windows') { go build -ldflags '-H=windowsgui' -o $output . } else { go build -o $output . }
    if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE" }
} finally {
    if ($null -ne $stagedResource -and (Test-Path -LiteralPath $stagedResource)) { Remove-Item -LiteralPath $stagedResource -Force }
    Pop-Location
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
}
if ($Target -eq 'wasm') {
    $web = Join-Path $bin 'web'
    if ($WebMode -eq 'apk') { $web = Join-Path $bin 'web-apk' }
    New-Item -ItemType Directory -Force -Path $web | Out-Null
    Copy-Item -LiteralPath $output -Destination (Join-Path $web 'aoz.wasm') -Force
    Copy-Item -LiteralPath (Join-Path $project 'web\index.html') -Destination (Join-Path $web 'index.html') -Force
    if (Test-Path -LiteralPath (Join-Path $project 'web\favicon.png')) { Copy-Item -LiteralPath (Join-Path $project 'web\favicon.png') -Destination (Join-Path $web 'favicon.png') -Force }
    if (Test-Path -LiteralPath (Join-Path $project 'web\favicon.ico')) { Copy-Item -LiteralPath (Join-Path $project 'web\favicon.ico') -Destination (Join-Path $web 'favicon.ico') -Force }
    $wasmExec = Join-Path (go env GOROOT) 'lib\wasm\wasm_exec.js'
    if (-not (Test-Path -LiteralPath $wasmExec)) { $wasmExec = Join-Path (go env GOROOT) 'misc\wasm\wasm_exec.js' }
    if (-not (Test-Path -LiteralPath $wasmExec)) { throw "wasm_exec.js not found below GoROOT" }
    Copy-Item -LiteralPath $wasmExec -Destination (Join-Path $web 'wasm_exec.js') -Force
    if ($WebMode -eq 'apk') {
        # No game data: the page asks for the visitor's APK. config.json switches the page to that mode, and its
        # cacheVersion (the build's hash) makes browsers rebuild their kept game files after an update.
        Copy-Item -LiteralPath (Join-Path $project 'web\memfs.js') -Destination (Join-Path $web 'memfs.js') -Force
        $version = (Get-FileHash -LiteralPath $output -Algorithm SHA256).Hash.Substring(0, 16).ToLower()
        Set-Content -LiteralPath (Join-Path $web 'config.json') -Value ('{"mode":"apk","cacheVersion":"' + $version + '"}') -Encoding ascii
        if (Test-Path -LiteralPath (Join-Path $web 'data')) { Remove-Item -LiteralPath (Join-Path $web 'data') -Recurse -Force }
    } else {
        if (Test-Path -LiteralPath (Join-Path $web 'config.json')) { Remove-Item -LiteralPath (Join-Path $web 'config.json') -Force }
        $cache = Join-Path $bin 'data-cache'
        if (Test-Path -LiteralPath $cache) { Copy-Item -LiteralPath $cache -Destination (Join-Path $web 'data') -Recurse -Force }
    }
}
Write-Output $output
