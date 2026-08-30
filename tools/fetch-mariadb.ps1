. "$PSScriptRoot\_common.ps1"

$envMap = Import-PortableEnv
$version = Get-EnvValue $envMap 'MARIADB_VERSION' '10.11.11'
$url = Get-EnvValue $envMap 'MARIADB_ZIP_URL' "https://archive.mariadb.org/mariadb-$version/winx64-packages/mariadb-$version-winx64.zip"

$root = $script:RepoRoot
$dest = Join-Path $root 'mariadb'
$cache = Join-Path $root 'tools\.cache'
$zipName = "mariadb-$version-winx64.zip"
$zipPath = Join-Path $cache $zipName

New-Item -ItemType Directory -Force -Path $cache | Out-Null

$existing = Find-MariaDbBin -Root $root
if ($existing) {
    Write-Host "MariaDB already at $existing"
    return
}

if (-not (Test-Path $zipPath)) {
    Write-Host "Downloading MariaDB $version from $url"
    Save-DownloadFileAtomic -Url $url -OutFile $zipPath
}

Write-Host "Unpacking..."
$extract = Join-Path $cache "extract-$version"
if (Test-Path $extract) { Remove-Item -LiteralPath $extract -Recurse -Force }
New-Item -ItemType Directory -Force -Path $extract | Out-Null
try {
    Expand-Archive -LiteralPath $zipPath -DestinationPath $extract -Force
    $inner = Get-ChildItem -LiteralPath $extract -Directory | Select-Object -First 1
    if (-not $inner) { throw "ZIP had no top-level folder under $extract" }

    if (Test-Path $dest) { Remove-Item -LiteralPath $dest -Recurse -Force }
    New-Item -ItemType Directory -Force -Path $dest | Out-Null
    Get-ChildItem -LiteralPath $inner.FullName | Move-Item -Destination $dest

    $bin = Find-MariaDbBin -Root $root
    if (-not $bin) { throw 'Unpack finished but mysqld/mariadbd is missing.' }
}
catch {
    Remove-Item -LiteralPath $zipPath -Force -ErrorAction SilentlyContinue
    throw
}
finally {
    Remove-Item -LiteralPath $extract -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host "OK: $bin"
