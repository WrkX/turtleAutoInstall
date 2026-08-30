param(
    [switch]$Force
)

. "$PSScriptRoot\_common.ps1"

$envMap = Import-PortableEnv
$root = $script:RepoRoot
$serverDir = Join-Path $root 'server'
$cache = Join-Path $root 'tools\.cache'
$marker = Join-Path $root 'data\.server-release'

New-Item -ItemType Directory -Force -Path $cache, $serverDir | Out-Null

if ((Test-Path (Join-Path $serverDir 'mangosd.exe')) -and
    (Test-Path (Join-Path $serverDir 'realmd.exe')) -and -not $Force) {
    Write-Host "Server already at $serverDir"
    return
}

$asset = Resolve-TortoiseWowReleaseAsset -EnvMap $envMap `
    -AssetNamePattern 'tortoise-wow-windows-server-*.zip' `
    -OverrideUrlKey 'TORTOISE_WOW_SERVER_ZIP_URL' `
    -FallbackZipName 'tortoise-wow-windows-server.zip' `
    -Kind 'server zip'

$zipPath = Join-Path $cache $asset.Name

if ($Force -or -not (Test-Path $zipPath)) {
    Write-Host "Downloading $($asset.Url)"
    Save-DownloadFileAtomic -Url $asset.Url -OutFile $zipPath
}

Write-Host "Unpacking into server\..."
$extract = Join-Path $cache 'server-extract'
$backup = Join-Path $cache ("server-backup-" + [guid]::NewGuid().ToString('N'))
if (Test-Path $extract) { Remove-Item -LiteralPath $extract -Recurse -Force }
New-Item -ItemType Directory -Force -Path $extract | Out-Null
$backedUp = @()
$installed = @()
$keepBackup = $false
try {
    Expand-Archive -LiteralPath $zipPath -DestinationPath $extract -Force
    $files = @(Get-ChildItem -LiteralPath $extract -File)
    foreach ($requiredExe in @('mangosd.exe', 'realmd.exe')) {
        if (-not ($files | Where-Object { $_.Name -ieq $requiredExe })) {
            throw "Unpack finished but the archive has no $requiredExe at its top level."
        }
    }
    New-Item -ItemType Directory -Force -Path $backup | Out-Null
    foreach ($file in $files) {
        $destination = Join-Path $serverDir $file.Name
        if (Test-Path $destination) {
            Move-Item -LiteralPath $destination -Destination (Join-Path $backup $file.Name)
            $backedUp += $file.Name
        }
    }
    foreach ($file in $files) {
        Move-Item -LiteralPath $file.FullName -Destination (Join-Path $serverDir $file.Name)
        $installed += $file.Name
    }
    foreach ($requiredExe in @('mangosd.exe', 'realmd.exe')) {
        if (-not (Test-Path (Join-Path $serverDir $requiredExe))) {
            throw "Unpack finished but server\$requiredExe is missing."
        }
    }
}
catch {
    foreach ($name in @($installed)) {
        $destination = Join-Path $serverDir $name
        if (Test-Path $destination) { Remove-Item -LiteralPath $destination -Force -ErrorAction SilentlyContinue }
    }
    foreach ($name in @($backedUp)) {
        $old = Join-Path $backup $name
        if (Test-Path $old) {
            try {
                Move-Item -LiteralPath $old -Destination (Join-Path $serverDir $name) -Force -ErrorAction Stop
            }
            catch {
                $keepBackup = $true
                Write-Warning "could not restore server\$name from rollback backup: $_"
            }
        }
    }
    if ($keepBackup) {
        Write-Warning "rollback was incomplete; preserving backup at $backup"
    }
    Remove-Item -LiteralPath $zipPath -Force -ErrorAction SilentlyContinue
    throw
}
finally {
    Remove-Item -LiteralPath $extract -Recurse -Force -ErrorAction SilentlyContinue
    if (-not $keepBackup) {
        Remove-Item -LiteralPath $backup -Recurse -Force -ErrorAction SilentlyContinue
    }
}

$dataDir = Join-Path $root 'data'
New-Item -ItemType Directory -Force -Path $dataDir | Out-Null
$tag = $asset.TagName
if (-not $tag) { $tag = $asset.Name }
Set-Content -LiteralPath $marker -Value $tag -Encoding ASCII

Write-Host "OK: $serverDir ($tag)"
