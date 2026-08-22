. "$PSScriptRoot\_common.ps1"

$envMap = Import-PortableEnv
$root = $script:RepoRoot
$sqlDir = Join-Path $root 'sql'
$cache = Join-Path $root 'tools\.cache'
$marker = Join-Path $sqlDir 'create_databases.sql'

New-Item -ItemType Directory -Force -Path $cache | Out-Null

if (Test-Path $marker) {
    Write-Host "SQL already at $sqlDir"
    exit 0
}

$asset = Resolve-TortoiseWowReleaseAsset -EnvMap $envMap `
    -AssetNamePattern 'tortoise-wow-sql-*.zip' `
    -OverrideUrlKey 'TORTOISE_WOW_SQL_ZIP_URL' `
    -FallbackZipName 'tortoise-wow-sql.zip' `
    -Kind 'SQL zip'

$zipPath = Join-Path $cache $asset.Name

if (-not (Test-Path $zipPath)) {
    Write-Host "Downloading $($asset.Url)"
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest -Uri $asset.Url -OutFile $zipPath
}

Write-Host "Unpacking into sql\..."
$extract = Join-Path $cache 'sql-extract'
if (Test-Path $extract) { Remove-Item -LiteralPath $extract -Recurse -Force }
New-Item -ItemType Directory -Force -Path $extract | Out-Null
Expand-Archive -LiteralPath $zipPath -DestinationPath $extract -Force

$srcSql = Join-Path $extract 'sql'
if (-not (Test-Path (Join-Path $srcSql 'create_databases.sql'))) {
    throw 'Unpack finished but sql\create_databases.sql is missing — bad zip layout?'
}

if (Test-Path $sqlDir) { Remove-Item -LiteralPath $sqlDir -Recurse -Force }
Move-Item -LiteralPath $srcSql -Destination $sqlDir
Remove-Item -LiteralPath $extract -Recurse -Force

Write-Host "OK: $sqlDir"
