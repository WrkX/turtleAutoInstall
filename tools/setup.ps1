param(
    [switch]$ForceReimport,
    [switch]$SkipDownload,
    [switch]$SkipSqlSync
)

. "$PSScriptRoot\_common.ps1"
$envMap = Import-PortableEnv
$root = $script:RepoRoot

$port = [int](Get-EnvValue $envMap 'MYSQL_PORT' '3307')
$rootUser = Get-EnvValue $envMap 'MYSQL_ROOT_USER' 'root'
$rootPass = Get-EnvValue $envMap 'MYSQL_ROOT_PASSWORD' ''
$user = Get-EnvValue $envMap 'MYSQL_USER' 'mangos'
$pass = Get-EnvValue $envMap 'MYSQL_PASSWORD' 'mangos'
$minBots = Get-EnvValue $envMap 'MIN_RANDOM_BOTS' '20'
$maxBots = Get-EnvValue $envMap 'MAX_RANDOM_BOTS' '20'
$worldPort = Get-EnvValue $envMap 'WORLD_PORT' '8090'
$realmPort = Get-EnvValue $envMap 'REALM_PORT' '3724'

$serverDir = Join-Path $root 'server'
$mapsDir = Join-Path $root 'maps'
$logsDir = Join-Path $root 'logs'
$dataMysql = Join-Path $root 'data\mysql'
$confDir = Join-Path $root 'conf'
$marker = Join-Path $root 'data\.setup-complete'
$incompleteMarker = Join-Path $root 'data\.setup-incomplete'

Write-Host "setup ($root)"

if (-not $SkipDownload) {
    & "$PSScriptRoot\fetch-server.ps1"
    & "$PSScriptRoot\fetch-sql.ps1"
    & "$PSScriptRoot\fetch-mariadb.ps1"
    & "$PSScriptRoot\fetch-maps.ps1"
}
elseif (-not (Test-Path (Join-Path $serverDir 'mangosd.exe'))) {
    Write-Warning 'no server\mangosd.exe - run without -SkipDownload or drop a build in server'
}

# A setup that completes without client data produces a server which starts
# only to fail when it first loads the world. Validate this after both normal
# downloads and -SkipDownload applies.
Assert-PortableMapsPresent -MapsDir $mapsDir

$bin = Find-MariaDbBin -Root $root
if (-not $bin) { throw 'MariaDB bin directory not found after fetch.' }
$mysqld = Get-MysqldPath -BinDir $bin
$client = Get-MysqlClientPath -BinDir $bin
$mariadbBasedir = Split-Path -Parent $bin

# my.ini
$myIni = Join-Path $confDir 'my.ini'
$tmpDir = Join-Path $root 'data\mysql-tmp'
New-Item -ItemType Directory -Force -Path $dataMysql, $tmpDir, $logsDir | Out-Null

Write-FromTemplate -TemplatePath (Join-Path $confDir 'my.ini.template') -OutPath $myIni -Replacements @{
    '@MYSQL_PORT@' = "$port"
    '@BASEDIR@'    = (Convert-ToIniPath $mariadbBasedir)
    '@DATADIR@'    = (Convert-ToIniPath $dataMysql)
    '@TMPDIR@'     = (Convert-ToIniPath $tmpDir)
}

if (-not (Test-MysqlDatadirInitialized -DataDir $dataMysql)) {
    Write-Host "init datadir"
    Initialize-MysqlDatadir -DataDir $dataMysql -BinDir $bin -Mysqld $mysqld -DefaultsFile $myIni -Port $port -RootPassword $rootPass
}
else {
    Write-Host "datadir already there, skipping init"
}

Assert-PortableMysqlPortAvailable -Port $port -MysqldPath $mysqld

$mysqlAlreadyReady = Test-MysqlReady -Client $client -User $rootUser -Password $rootPass -Port $port -BinDir $bin -DefaultsFile $myIni
$mysqlListener = Get-ListeningProcessForPort -Port $port
$mysqlAlreadyRunning = ($mysqlListener -and
    (Test-PortableProcessId -ProcessId ([int]$mysqlListener.ProcessId) -ExecutablePath $mysqld))
$startedHere = (-not $mysqlAlreadyReady) -and (-not $mysqlAlreadyRunning)
try {
    if (-not $mysqlAlreadyReady) {
        # Use the normal launcher so a readiness failure cannot leave an
        # untracked mysqld process behind before this try/finally can clean up.
        & "$PSScriptRoot\start-mysql.ps1"
    }

    # SQL - release zip above; local checkout fallback
    $createSql = Join-Path $root 'sql\create_databases.sql'
    if (-not (Test-Path $createSql) -and -not $SkipSqlSync) {
        Write-Host "sql\ still empty, syncing from source tree"
        & "$PSScriptRoot\sync-sql.ps1"
    }

    if ((Test-Path $marker) -and -not $ForceReimport) {
        Write-Host "already imported ($marker) - pass -ForceReimport to wipe and reload"
    }
    else {
        $recoveringImport = Test-Path $incompleteMarker
        Set-Content -LiteralPath $incompleteMarker -Value (Get-Date -Format 'o') -Encoding ASCII
        Remove-Item -LiteralPath $marker -Force -ErrorAction SilentlyContinue
        if ($ForceReimport -or $recoveringImport) {
            Write-Host "dropping tw_* databases"
            $dropSql = 'DROP DATABASE IF EXISTS tw_world; DROP DATABASE IF EXISTS tw_char; DROP DATABASE IF EXISTS tw_logon; DROP DATABASE IF EXISTS tw_logs;'
            Invoke-Mysql -Client $client -User $rootUser -Password $rootPass -Port $port -Execute $dropSql -BinDir $bin -DefaultsFile $myIni
        }
        try {
            & "$PSScriptRoot\import-databases.ps1"
            Set-Content -LiteralPath $marker -Value (Get-Date -Format 'o') -Encoding ASCII
            Remove-Item -LiteralPath $incompleteMarker -Force -ErrorAction SilentlyContinue
        }
        catch {
            # Keep .setup-incomplete so a rerun first clears any partially
            # imported databases instead of layering duplicate data over them.
            Remove-Item -LiteralPath $marker -Force -ErrorAction SilentlyContinue
            throw
        }
    }

    # confs from .dist beside the binaries
    function Resolve-Dist {
        param([string]$Name)
        $candidates = @(
            (Join-Path $serverDir $Name),
            (Join-Path $serverDir ($Name -replace '\.dist$', '.dist.in')),
            (Join-Path $confDir $Name)
        )
        foreach ($c in $candidates) {
            if (Test-Path $c) { return $c }
        }
        return $null
    }

    $pairs = @(
        @{ Dist = 'mangosd.conf.dist'; Out = (Join-Path $serverDir 'mangosd.conf') },
        @{ Dist = 'realmd.conf.dist';   Out = (Join-Path $serverDir 'realmd.conf') },
        @{ Dist = 'aiplayerbot.conf.dist'; Out = (Join-Path $serverDir 'aiplayerbot.conf') },
        @{ Dist = 'ahbot.conf.dist';    Out = (Join-Path $serverDir 'ahbot.conf') }
    )

    foreach ($p in $pairs) {
        $dist = Resolve-Dist $p.Dist
        if (-not $dist) {
            Write-Warning "no $($p.Dist), skipping $(Split-Path -Leaf $p.Out)"
            continue
        }
        Copy-Item -LiteralPath $dist -Destination $p.Out -Force
    }

    $mangosdConf = Join-Path $serverDir 'mangosd.conf'
    $realmdConf = Join-Path $serverDir 'realmd.conf'
    $botConf = Join-Path $serverDir 'aiplayerbot.conf'

    if (Test-Path $mangosdConf) {
        Patch-ConfDatabaseLines -Path $mangosdConf -DbHost '127.0.0.1' -Port $port -User $user -Password $pass
        Set-ConfValue -Path $mangosdConf -Key 'DataDir' -Value ('"' + (Convert-ToIniPath $mapsDir) + '"')
        Set-ConfValue -Path $mangosdConf -Key 'LogsDir' -Value ('"' + (Convert-ToIniPath $logsDir) + '"')
        Set-ConfValue -Path $mangosdConf -Key 'WorldServerPort' -Value $worldPort
        Set-ConfValue -Path $mangosdConf -Key 'Database.AutoUpdate.Enabled' -Value '1'
        $updatesPath = Convert-ToIniPath (Join-Path $root 'sql\database_updates')
        Set-ConfValue -Path $mangosdConf -Key 'Database.AutoUpdate.Path' -Value ('"' + $updatesPath + '/"')
        Set-ConfValue -Path $mangosdConf -Key 'LogSQL' -Value '0'
        Set-ConfValue -Path $mangosdConf -Key 'LFT.BotFill.Enable' -Value '1'
        Set-ConfValue -Path $mangosdConf -Key 'SoloDungeonRepopAlive.Enable' -Value '1'
        Set-ConfValue -Path $mangosdConf -Key 'Leech.Enable' -Value '1'
    }
    if (Test-Path $realmdConf) {
        Patch-ConfDatabaseLines -Path $realmdConf -DbHost '127.0.0.1' -Port $port -User $user -Password $pass
        Set-ConfValue -Path $realmdConf -Key 'LogsDir' -Value ('"' + (Convert-ToIniPath $logsDir) + '/"')
        Set-ConfValue -Path $realmdConf -Key 'RealmServerPort' -Value $realmPort
    }
    if (Test-Path $botConf) {
        Set-ConfValue -Path $botConf -Key 'AiPlayerbot.Enabled' -Value '1'
        Set-ConfValue -Path $botConf -Key 'AiPlayerbot.RandomBotAutoCreate' -Value '1'
        Set-ConfValue -Path $botConf -Key 'AiPlayerbot.DeleteRandomBotAccounts' -Value '0'
        Set-ConfValue -Path $botConf -Key 'AiPlayerbot.MinRandomBots' -Value $minBots
        Set-ConfValue -Path $botConf -Key 'AiPlayerbot.MaxRandomBots' -Value $maxBots
    }

    Write-Host "syncing database credentials for $user"
    Ensure-PortableDatabaseUser -Client $client -BinDir $bin -DefaultsFile $myIni `
        -RootUser $rootUser -RootPass $rootPass -Port $port -User $user -Password $pass

    # Keep the login database in lockstep with the values just written to the
    # conf files, including when the import marker caused SQL loading to skip.
    & "$PSScriptRoot\sync-realmlist.ps1"
}
finally {
    if ($startedHere) {
        Write-Host "stopping mysqld"
        & "$PSScriptRoot\stop-mysql.ps1"
    }
}

Write-Host 'done. run start.bat when the server and maps are ready'
Write-Host 'create an account with tools\create-account.ps1 -Username <name> (password prompt)'
