param(
    [switch]$Force
)

. "$PSScriptRoot\_common.ps1"

$envMap = Import-PortableEnv
$root = $script:RepoRoot
$sqlDir = Join-Path $root 'sql'
$cache = Join-Path $root 'tools\.cache'
$haveSql = Join-Path $sqlDir 'create_databases.sql'
$marker = Join-Path $root 'data\.sql-release'

New-Item -ItemType Directory -Force -Path $cache | Out-Null

if ((Test-Path $haveSql) -and -not $Force) {
    Write-Host "SQL already at $sqlDir (pass -Force to replace from GitHub)"
    return
}

$asset = Resolve-TortoiseWowReleaseAsset -EnvMap $envMap `
    -AssetNamePattern 'tortoise-wow-sql-*.zip' `
    -OverrideUrlKey 'TORTOISE_WOW_SQL_ZIP_URL' `
    -FallbackZipName 'tortoise-wow-sql.zip' `
    -Kind 'SQL zip'

$zipPath = Join-Path $cache $asset.Name

if ($Force -or -not (Test-Path $zipPath)) {
    Write-Host "Downloading $($asset.Url)"
    Save-DownloadFileAtomic -Url $asset.Url -OutFile $zipPath
}

Write-Host "Unpacking into sql\..."
$extract = Join-Path $cache 'sql-extract'
$backup = Join-Path $cache ("sql-backup-" + [guid]::NewGuid().ToString('N'))
if (Test-Path $extract) { Remove-Item -LiteralPath $extract -Recurse -Force }
New-Item -ItemType Directory -Force -Path $extract | Out-Null
$hadOldSql = $false
$newSqlMoved = $false
$keepBackup = $false
try {
    Expand-Archive -LiteralPath $zipPath -DestinationPath $extract -Force
    $srcSql = Join-Path $extract 'sql'
    if (-not (Test-Path (Join-Path $srcSql 'create_databases.sql'))) {
        throw 'Unpack finished but sql\create_databases.sql is missing - bad zip layout?'
    }
    # Keep the current checkout until the replacement has been validated and
    # moved successfully. A corrupt/partial update must not strand setup with
    # no SQL at all.
    if (Test-Path $sqlDir) {
        Move-Item -LiteralPath $sqlDir -Destination $backup
        $hadOldSql = $true
    }
    Move-Item -LiteralPath $srcSql -Destination $sqlDir
    $newSqlMoved = $true
}
catch {
    if ($newSqlMoved -and (Test-Path $sqlDir)) {
        Remove-Item -LiteralPath $sqlDir -Recurse -Force -ErrorAction SilentlyContinue
    }
    if ($hadOldSql -and (Test-Path $backup)) {
        try {
            Move-Item -LiteralPath $backup -Destination $sqlDir -Force -ErrorAction Stop
        }
        catch {
            $keepBackup = $true
            Write-Warning "could not restore previous SQL tree: $_"
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

Write-Host "OK: $sqlDir ($tag)"
