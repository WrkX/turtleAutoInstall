param(
    [switch]$ForceReimport
)

. "$PSScriptRoot\_common.ps1"

Write-Host "update ($($script:RepoRoot))"

function Invoke-UpdateStep {
    param(
        [string]$Path,
        # Hashtable splat so -Force/-SkipDownload bind as named switches.
        # An array splat like @('-Force') is positional and leaves [switch]
        # parameters off, so fetch-server/sql skip an existing install.
        [hashtable]$NamedArgs,
        [string]$Label = (Split-Path -Leaf $Path)
    )
    if (-not $NamedArgs) { $NamedArgs = @{} }
    # Native commands can leave a stale $LASTEXITCODE behind when a nested
    # PowerShell script succeeds. Treat terminating errors and this invocation's
    # success state as authoritative instead.
    $global:LASTEXITCODE = 0
    & $Path @NamedArgs
    if (-not $?) { throw "$Label failed" }
}

Write-Host "stopping realm if running"
Invoke-UpdateStep -Path "$PSScriptRoot\stop.ps1" -Label 'stop'

Invoke-UpdateStep -Path "$PSScriptRoot\fetch-server.ps1" -NamedArgs @{ Force = $true } -Label 'fetch-server'

Invoke-UpdateStep -Path "$PSScriptRoot\fetch-sql.ps1" -NamedArgs @{ Force = $true } -Label 'fetch-sql'

# Refresh the maps URL and replace installed data only when its stable source
# identity changed. Run fetch-maps.ps1 -Force if content changed in place.
Invoke-UpdateStep -Path "$PSScriptRoot\fetch-maps.ps1" -Label 'fetch-maps'

$setupArgs = @{ SkipDownload = $true }
if ($ForceReimport) {
    $setupArgs['ForceReimport'] = $true
}
Write-Host "rewriting confs (database kept unless -ForceReimport)"
Invoke-UpdateStep -Path "$PSScriptRoot\setup.ps1" -NamedArgs $setupArgs -Label 'setup'

Write-Host 'update done. start.bat when you want the realm back.'
