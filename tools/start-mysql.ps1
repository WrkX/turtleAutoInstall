. "$PSScriptRoot\_common.ps1"
$envMap = Import-PortableEnv
$root = $script:RepoRoot

$port = [int](Get-EnvValue $envMap 'MYSQL_PORT' '3307')
$rootUser = Get-EnvValue $envMap 'MYSQL_ROOT_USER' 'root'
$rootPass = Get-EnvValue $envMap 'MYSQL_ROOT_PASSWORD' ''

$bin = Find-MariaDbBin -Root $root
if (-not $bin) { throw 'MariaDB missing - run setup.bat first.' }
$mysqld = Get-MysqldPath -BinDir $bin
$client = Get-MysqlClientPath -BinDir $bin
$myIni = Join-Path $root 'conf\my.ini'
if (-not (Test-Path $myIni)) { throw "no $myIni - run setup.bat first." }

Assert-PortableMysqlPortAvailable -Port $port -MysqldPath $mysqld

if (Test-MysqlReady -Client $client -User $rootUser -Password $rootPass -Port $port -BinDir $bin -DefaultsFile $myIni) {
    Write-Host "mysqld already up on $port"
    return
}

$listener = Get-ListeningProcessForPort -Port $port
if ($listener -and (Test-PortableProcessId -ProcessId ([int]$listener.ProcessId) -ExecutablePath $mysqld)) {
    throw "portable mysqld is already listening on $port but rejected the configured credentials; check MYSQL_ROOT_USER/MYSQL_ROOT_PASSWORD"
}

Write-Host "starting mysqld"
$proc = Start-Process -FilePath $mysqld -ArgumentList "--defaults-file=$myIni" -PassThru -WindowStyle Minimized
Write-PortablePid -ProcessName 'mysqld' -ProcessId $proc.Id
try {
    Wait-MysqlReady -Client $client -User $rootUser -Password $rootPass -Port $port -BinDir $bin -DefaultsFile $myIni -TimeoutSec 90
}
catch {
    # Do not leave a broken daemon (or a PID file pointing at it) behind when
    # authentication/configuration prevents startup from becoming ready.
    if (Test-PortableProcessId -ProcessId $proc.Id -ExecutablePath $mysqld) {
        Write-Warning "mysqld did not become ready; stopping pid $($proc.Id)"
        Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
    }
    Remove-PortablePid -ProcessName 'mysqld'
    throw
}
Write-Host "mysqld pid $($proc.Id)"
