. "$PSScriptRoot\_common.ps1"

foreach ($name in @('mangosd', 'realmd')) {
    Get-Process -Name $name -ErrorAction SilentlyContinue | ForEach-Object {
        Write-Host "kill $($_.ProcessName) $($_.Id)"
        Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue
    }
}

& "$PSScriptRoot\stop-mysql.ps1"
