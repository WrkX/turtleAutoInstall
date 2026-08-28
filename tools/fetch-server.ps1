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

if ((Test-Path (Join-Path $serverDir 'mangosd.exe')) -and -not $Force) {
    Write-Host "Server already at $serverDir"
    exit 0
}

$asset = Resolve-TortoiseWowReleaseAsset -EnvMap $envMap `
    -AssetNamePattern 'tortoise-wow-windows-server-*.zip' `
    -OverrideUrlKey 'TORTOISE_WOW_SERVER_ZIP_URL' `
    -FallbackZipName 'tortoise-wow-windows-server.zip' `
    -Kind 'server zip'

$zipPath = Join-Path $cache $asset.Name

if (-not (Test-Path $zipPath)) {
    Write-Host "Downloading $($asset.Url)"
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest -Uri $asset.Url -OutFile $zipPath
}
elseif ($Force) {
    Write-Host "Using cached $($asset.Name)"
}

Write-Host "Unpacking into server\..."
$extract = Join-Path $cache 'server-extract'
if (Test-Path $extract) { Remove-Item -LiteralPath $extract -Recurse -Force }
New-Item -ItemType Directory -Force -Path $extract | Out-Null
Expand-Archive -LiteralPath $zipPath -DestinationPath $extract -Force

Get-ChildItem -LiteralPath $extract -File | Move-Item -Destination $serverDir -Force
Remove-Item -LiteralPath $extract -Recurse -Force

if (-not (Test-Path (Join-Path $serverDir 'mangosd.exe'))) {
    throw 'Unpack finished but server\mangosd.exe is missing.'
}

$dataDir = Join-Path $root 'data'
New-Item -ItemType Directory -Force -Path $dataDir | Out-Null
$tag = $asset.TagName
if (-not $tag) { $tag = $asset.Name }
Set-Content -LiteralPath $marker -Value $tag -Encoding ASCII

Write-Host "OK: $serverDir ($tag)"
