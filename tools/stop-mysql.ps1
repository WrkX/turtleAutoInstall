. "$PSScriptRoot\_common.ps1"
$envMap = Import-PortableEnv
$root = $script:RepoRoot

$port = [int](Get-EnvValue $envMap 'MYSQL_PORT' '3307')
$rootUser = Get-EnvValue $envMap 'MYSQL_ROOT_USER' 'root'
$rootPass = Get-EnvValue $envMap 'MYSQL_ROOT_PASSWORD' ''

$bin = Find-MariaDbBin -Root $root
if (-not $bin) {
    Write-Host 'no mariadb folder'
    return
}
$client = Get-MysqlClientPath -BinDir $bin
$myIni = Join-Path $root 'conf\my.ini'
$mysqld = Get-MysqldPath -BinDir $bin

# Never send SHUTDOWN to an unrelated MySQL instance that happens to use the
# configured port. Confirm the listener (or recorded pid) belongs to this
# portable installation first.
$owner = Get-ListeningProcessForPort -Port $port
$ownerIsOurs = $false
if ($owner -and $owner.ExecutablePath) {
    $ownerIsOurs = (Resolve-ProcessExecutablePath -Path ([string]$owner.ExecutablePath)) -eq (Resolve-ProcessExecutablePath -Path $mysqld)
}
$recordedPid = Read-PortablePid -ProcessName 'mysqld'
if (-not $ownerIsOurs) {
    # If listener ownership cannot be established, use only the PID explicitly
    # recorded by this install. A shared executable can serve another datadir,
    # so killing every exact-path match would still be too broad.
    $portableIds = @()
    if ($recordedPid -and (Test-PortableProcessId -ProcessId $recordedPid -ExecutablePath $mysqld)) {
        $portableIds += $recordedPid
    }
    $portableIds = @($portableIds | Sort-Object -Unique)
    foreach ($portableId in $portableIds) {
        Write-Host "stopping portable mysqld pid $portableId"
        Stop-Process -Id $portableId -Force -ErrorAction SilentlyContinue
    }
    Remove-PortablePid -ProcessName 'mysqld'
    if ($portableIds.Count -eq 0) {
        Write-Warning "could not verify ownership of the process listening on $port; nothing was stopped"
    }
    return
}

if (-not (Test-MysqlReady -Client $client -User $rootUser -Password $rootPass -Port $port -BinDir $bin -DefaultsFile $myIni)) {
    Write-Warning "portable mysqld owns port $port but rejected the configured credentials; stopping the verified process"
    $verifiedPid = [int]$owner.ProcessId
    if (Test-PortableProcessId -ProcessId $verifiedPid -ExecutablePath $mysqld) {
        Stop-Process -Id $verifiedPid -Force -ErrorAction SilentlyContinue
    }
    Remove-PortablePid -ProcessName 'mysqld'
    return
}

Write-Host "SHUTDOWN on $port"
try {
    Invoke-Mysql -Client $client -User $rootUser -Password $rootPass -Port $port -BinDir $bin -DefaultsFile $myIni -Execute 'SHUTDOWN;'
}
catch {
    Write-Warning "SHUTDOWN failed ($_) - killing the verified portable process"
    $killPid = [int]$owner.ProcessId
    if (Test-PortableProcessId -ProcessId $killPid -ExecutablePath $mysqld) {
        Stop-Process -Id $killPid -Force -ErrorAction SilentlyContinue
    }
}

$deadline = (Get-Date).AddSeconds(30)
while ((Get-Date) -lt $deadline) {
    if (-not (Test-MysqlReady -Client $client -User $rootUser -Password $rootPass -Port $port -BinDir $bin -DefaultsFile $myIni)) {
        Write-Host 'stopped'
        Remove-PortablePid -ProcessName 'mysqld'
        return
    }
    Start-Sleep -Seconds 1
}
Write-Warning 'still up after 30s'
