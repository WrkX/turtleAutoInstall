. "$PSScriptRoot\_common.ps1"
$root = $script:RepoRoot
$serverDir = Join-Path $root 'server'

if (-not (Test-Path (Join-Path $serverDir 'mangosd.exe'))) {
    throw "no server\mangosd.exe"
}
if (-not (Test-Path (Join-Path $serverDir 'realmd.exe'))) {
    throw "no server\realmd.exe"
}
if (-not (Test-Path (Join-Path $serverDir 'realmd.conf'))) {
    throw "no server\realmd.conf - run setup.bat"
}
if (-not (Test-Path (Join-Path $serverDir 'mangosd.conf'))) {
    throw "no server\mangosd.conf - run setup.bat"
}
Assert-PortableMapsPresent -MapsDir (Join-Path $root 'maps')

foreach ($spec in @(
    @{ Name = 'realmd'; Path = (Join-Path $serverDir 'realmd.exe') },
    @{ Name = 'mangosd'; Path = (Join-Path $serverDir 'mangosd.exe') }
)) {
    $running = @(Get-PortableProcessesByPath -ExecutablePath $spec.Path)
    if ($running.Count -gt 0) {
        throw "$($spec.Name) is already running for this install (pid $($running[0].ProcessId))."
    }
    $recordedPid = Read-PortablePid -ProcessName $spec.Name
    if ($recordedPid -and (Test-PortableProcessId -ProcessId $recordedPid -ExecutablePath $spec.Path)) {
        throw "$($spec.Name) is already running for this install (pid $recordedPid)."
    }
}

$mysqlBin = Find-MariaDbBin -Root $root
if (-not $mysqlBin) { throw 'MariaDB missing - run setup.bat first.' }
$mysqldPath = Get-MysqldPath -BinDir $mysqlBin
$mysqlPidBefore = Read-PortablePid -ProcessName 'mysqld'
$mysqlWasRecorded = $mysqlPidBefore -and (Test-PortableProcessId -ProcessId $mysqlPidBefore -ExecutablePath $mysqldPath)

& "$PSScriptRoot\start-mysql.ps1"
$mysqlPidAfter = Read-PortablePid -ProcessName 'mysqld'
$startedMysqlHere = ((-not $mysqlWasRecorded) -and $mysqlPidAfter -and
    (Test-PortableProcessId -ProcessId $mysqlPidAfter -ExecutablePath $mysqldPath))

$started = @()
try {
    Write-Host "realmd"
    $realmdProc = Start-Process -FilePath (Join-Path $serverDir 'realmd.exe') -WorkingDirectory $serverDir -PassThru
    Write-PortablePid -ProcessName 'realmd' -ProcessId $realmdProc.Id
    $started += @{ Name = 'realmd'; Path = (Join-Path $serverDir 'realmd.exe'); Id = $realmdProc.Id }
    Start-Sleep -Seconds 2
    if ($realmdProc.HasExited) {
        throw "realmd exited during startup (code $($realmdProc.ExitCode))"
    }

    Write-Host "mangosd"
    $mangosdProc = Start-Process -FilePath (Join-Path $serverDir 'mangosd.exe') -WorkingDirectory $serverDir -PassThru
    Write-PortablePid -ProcessName 'mangosd' -ProcessId $mangosdProc.Id
    $started += @{ Name = 'mangosd'; Path = (Join-Path $serverDir 'mangosd.exe'); Id = $mangosdProc.Id }
    Start-Sleep -Seconds 2
    if ($mangosdProc.HasExited) {
        throw "mangosd exited during startup (code $($mangosdProc.ExitCode))"
    }
}
catch {
    # A half-started pair is more confusing than a clear failure and can leave
    # the next run blocked by a stale process. Only terminate the exact PIDs
    # launched above, after verifying their executable paths.
    for ($index = $started.Count - 1; $index -ge 0; $index--) {
        $item = $started[$index]
        if (Test-PortableProcessId -ProcessId $item.Id -ExecutablePath $item.Path) {
            Write-Warning "stopping $($item.Name) after startup failure"
            Stop-Process -Id $item.Id -Force -ErrorAction SilentlyContinue
        }
        Remove-PortablePid -ProcessName $item.Name
    }
    if ($startedMysqlHere -and (Test-PortableProcessId -ProcessId $mysqlPidAfter -ExecutablePath $mysqldPath)) {
        Write-Warning 'stopping MySQL started by this failed realm launch'
        Stop-Process -Id $mysqlPidAfter -Force -ErrorAction SilentlyContinue
        Remove-PortablePid -ProcessName 'mysqld'
    }
    throw
}

$envMap = Import-PortableEnv
$realmAddress = Get-EnvValue $envMap 'REALM_ADDRESS' '127.0.0.1'
Write-Host "create account: tools\create-account.ps1 -Username <name> (password prompt)"
Write-Host "then set realmlist.wtf -> $realmAddress"
