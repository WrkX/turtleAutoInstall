. "$PSScriptRoot\_common.ps1"
$root = $script:RepoRoot
$serverDir = Join-Path $root 'server'

if (-not (Test-Path (Join-Path $serverDir 'mangosd.exe'))) {
    throw "no server\mangosd.exe"
}
if (-not (Test-Path (Join-Path $serverDir 'realmd.exe'))) {
    throw "no server\realmd.exe"
}
if (-not (Test-Path (Join-Path $serverDir 'mangosd.conf'))) {
    throw "no server\mangosd.conf - run setup.bat"
}

& "$PSScriptRoot\start-mysql.ps1"

Write-Host "realmd"
Start-Process -FilePath (Join-Path $serverDir 'realmd.exe') -WorkingDirectory $serverDir
Start-Sleep -Seconds 2
Write-Host "mangosd"
Start-Process -FilePath (Join-Path $serverDir 'mangosd.exe') -WorkingDirectory $serverDir

Write-Host "account create <user> <pass>  (then realmlist.wtf -> 127.0.0.1)"
