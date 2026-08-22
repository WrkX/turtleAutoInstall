. "$PSScriptRoot\_common.ps1"
$envMap = Import-PortableEnv
$root = $script:RepoRoot

$port = [int](Get-EnvValue $envMap 'MYSQL_PORT' '3306')
$rootUser = Get-EnvValue $envMap 'MYSQL_ROOT_USER' 'root'
$rootPass = Get-EnvValue $envMap 'MYSQL_ROOT_PASSWORD' ''

$bin = Find-MariaDbBin -Root $root
if (-not $bin) { throw 'MariaDB missing - run setup.bat first.' }
$mysqld = Get-MysqldPath -BinDir $bin
$client = Get-MysqlClientPath -BinDir $bin
$myIni = Join-Path $root 'conf\my.ini'
if (-not (Test-Path $myIni)) { throw "no $myIni - run setup.bat first." }

if (Test-MysqlReady -Client $client -User $rootUser -Password $rootPass -Port $port) {
    Write-Host "mysqld already up on $port"
    exit 0
}

$pidFile = Join-Path $root 'data\mysqld.pid'
Write-Host "starting mysqld"
$proc = Start-Process -FilePath $mysqld -ArgumentList "--defaults-file=$myIni" -PassThru -WindowStyle Minimized
Set-Content -LiteralPath $pidFile -Value $proc.Id -Encoding ASCII
Wait-MysqlReady -Client $client -User $rootUser -Password $rootPass -Port $port -TimeoutSec 90
Write-Host "mysqld pid $($proc.Id)"
