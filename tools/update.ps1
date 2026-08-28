param(
    [switch]$ForceReimport
)

. "$PSScriptRoot\_common.ps1"

Write-Host "update ($($script:RepoRoot))"
Write-Host "stopping realm if running"
& "$PSScriptRoot\stop.ps1"

& "$PSScriptRoot\fetch-server.ps1" -Force
if (-not $? -or $LASTEXITCODE) { throw "fetch-server failed ($LASTEXITCODE)" }

& "$PSScriptRoot\fetch-sql.ps1" -Force
if (-not $? -or $LASTEXITCODE) { throw "fetch-sql failed ($LASTEXITCODE)" }

$setupArgs = @('-SkipDownload')
if ($ForceReimport) {
    $setupArgs += '-ForceReimport'
}
Write-Host "rewriting confs (database kept unless -ForceReimport)"
& "$PSScriptRoot\setup.ps1" @setupArgs
if (-not $? -or $LASTEXITCODE) { throw "setup failed ($LASTEXITCODE)" }

Write-Host 'update done. start.bat when you want the realm back.'
