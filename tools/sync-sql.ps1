param(
    [string]$Source = ''
)

. "$PSScriptRoot\_common.ps1"
$envMap = Import-PortableEnv
$root = $script:RepoRoot

if (-not $Source) {
    $Source = Get-EnvValue $envMap 'TORTOISE_WOW_SRC' ''
}
if (-not $Source) {
    $sibling = Join-Path (Split-Path -Parent $root) 'tortoise-wow'
    if (Test-Path (Join-Path $sibling 'sql\create_databases.sql')) {
        $Source = $sibling
    }
}
if (-not $Source -or -not (Test-Path $Source)) {
    throw "Pass -Source path\to\tortoise-wow or set TORTOISE_WOW_SRC in portable.local.env"
}

$srcSql = Join-Path $Source 'sql'
$dstSql = Join-Path $root 'sql'
if (-not (Test-Path (Join-Path $srcSql 'create_databases.sql'))) {
    throw "Not a tortoise-wow tree (missing sql\create_databases.sql): $Source"
}

Write-Host "Copying SQL from $Source"

New-Item -ItemType Directory -Force -Path $dstSql | Out-Null
Copy-Item -LiteralPath (Join-Path $srcSql 'create_databases.sql') -Destination $dstSql -Force

foreach ($dir in @('base', 'database_updates')) {
    $from = Join-Path $srcSql $dir
    $to = Join-Path $dstSql $dir
    if (-not (Test-Path $from)) {
        Write-Warning "Missing $from — skipped"
        continue
    }
    if (Test-Path $to) { Remove-Item -LiteralPath $to -Recurse -Force }
    Copy-Item -LiteralPath $from -Destination $to -Recurse -Force
}

$pbSrc = Join-Path $Source 'src\modules\PlayerBots\sql'
$pbDst = Join-Path $dstSql 'playerbots'
if (Test-Path $pbSrc) {
    if (Test-Path $pbDst) { Remove-Item -LiteralPath $pbDst -Recurse -Force }
    New-Item -ItemType Directory -Force -Path $pbDst | Out-Null

    $worldDst = Join-Path $pbDst 'world'
    New-Item -ItemType Directory -Force -Path $worldDst | Out-Null
    Copy-Item -Path (Join-Path $pbSrc 'world\*.sql') -Destination $worldDst -Force -ErrorAction SilentlyContinue
    $classic = Join-Path $pbSrc 'world\classic'
    if (Test-Path $classic) {
        Copy-Item -LiteralPath $classic -Destination (Join-Path $worldDst 'classic') -Recurse -Force
    }

    $charDst = Join-Path $pbDst 'characters'
    New-Item -ItemType Directory -Force -Path $charDst | Out-Null
    Copy-Item -Path (Join-Path $pbSrc 'characters\*.sql') -Destination $charDst -Force -ErrorAction SilentlyContinue
}
else {
    Write-Warning "No PlayerBots SQL at $pbSrc (bots will blow up on start without those tables)"
}

Write-Host "Done."
