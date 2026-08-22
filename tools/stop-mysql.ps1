. "$PSScriptRoot\_common.ps1"
$envMap = Import-PortableEnv
$root = $script:RepoRoot

$port = [int](Get-EnvValue $envMap 'MYSQL_PORT' '3306')
$rootUser = Get-EnvValue $envMap 'MYSQL_ROOT_USER' 'root'
$rootPass = Get-EnvValue $envMap 'MYSQL_ROOT_PASSWORD' ''

$bin = Find-MariaDbBin -Root $root
if (-not $bin) {
    Write-Host 'no mariadb folder'
    exit 0
}
$client = Get-MysqlClientPath -BinDir $bin

if (-not (Test-MysqlReady -Client $client -User $rootUser -Password $rootPass -Port $port)) {
    Write-Host "nothing listening on $port"
    exit 0
}

Write-Host "SHUTDOWN on $port"
try {
    Invoke-Mysql -Client $client -User $rootUser -Password $rootPass -Port $port -Execute 'SHUTDOWN;'
}
catch {
    Write-Warning "SHUTDOWN failed ($_) - killing recorded pid"
    $pidFile = Join-Path $root 'data\mysqld.pid'
    if (Test-Path $pidFile) {
        $procId = (Get-Content -LiteralPath $pidFile -Raw).Trim()
        if ($procId) {
            Stop-Process -Id ([int]$procId) -Force -ErrorAction SilentlyContinue
        }
    }
}

$deadline = (Get-Date).AddSeconds(30)
while ((Get-Date) -lt $deadline) {
    if (-not (Test-MysqlReady -Client $client -User $rootUser -Password $rootPass -Port $port)) {
        Write-Host 'stopped'
        exit 0
    }
    Start-Sleep -Seconds 1
}
Write-Warning 'still up after 30s'
