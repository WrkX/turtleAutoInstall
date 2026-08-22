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
    exit 0
}

if (-not (Test-Path $zipPath)) {
    Write-Host "Downloading MariaDB $version from $url"
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest -Uri $url -OutFile $zipPath
}

Write-Host "Unpacking..."
if (Test-Path $dest) {
    Remove-Item -LiteralPath $dest -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $dest | Out-Null

$extract = Join-Path $cache "extract-$version"
if (Test-Path $extract) { Remove-Item -LiteralPath $extract -Recurse -Force }
New-Item -ItemType Directory -Force -Path $extract | Out-Null
Expand-Archive -LiteralPath $zipPath -DestinationPath $extract -Force

$inner = Get-ChildItem -LiteralPath $extract -Directory | Select-Object -First 1
if (-not $inner) { throw "ZIP had no top-level folder under $extract" }

Get-ChildItem -LiteralPath $inner.FullName | Move-Item -Destination $dest
Remove-Item -LiteralPath $extract -Recurse -Force

$bin = Find-MariaDbBin -Root $root
if (-not $bin) { throw 'Unpack finished but mysqld/mariadbd is missing.' }

Write-Host "OK: $bin"
