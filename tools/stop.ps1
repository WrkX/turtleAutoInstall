. "$PSScriptRoot\_common.ps1"

foreach ($spec in @(
    @{ Name = 'mangosd'; Path = (Join-Path $script:RepoRoot 'server\mangosd.exe') },
    @{ Name = 'realmd'; Path = (Join-Path $script:RepoRoot 'server\realmd.exe') }
)) {
    $ids = @((Get-PortableProcessesByPath -ExecutablePath $spec.Path | ForEach-Object { [int]$_.ProcessId }) |
        Where-Object { $_ -gt 0 })
    $recordedPid = Read-PortablePid -ProcessName $spec.Name
    if ($recordedPid -and (Test-PortableProcessId -ProcessId $recordedPid -ExecutablePath $spec.Path)) {
        $ids += $recordedPid
    }
    $ids = @($ids | Sort-Object -Unique)
    foreach ($id in $ids) {
        Write-Host "kill $($spec.Name) $id"
        Stop-Process -Id $id -Force -ErrorAction SilentlyContinue
    }
    $alive = @()
    if ($ids.Count -gt 0) {
        $deadline = (Get-Date).AddSeconds(10)
        do {
            Start-Sleep -Milliseconds 200
            $alive = @($ids | Where-Object { Get-Process -Id $_ -ErrorAction SilentlyContinue })
        } while ($alive.Count -gt 0 -and (Get-Date) -lt $deadline)
        if ($alive.Count -gt 0) { Write-Warning "$($spec.Name) did not exit after 10 seconds" }
    }
    if (-not $alive -or $alive.Count -eq 0) { Remove-PortablePid -ProcessName $spec.Name }
}

& "$PSScriptRoot\stop-mysql.ps1"
