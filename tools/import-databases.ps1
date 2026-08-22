param(
    [switch]$SkipBase,
    [switch]$SkipUpdates,
    [switch]$SkipPlayerbots
)

. "$PSScriptRoot\_common.ps1"
$envMap = Import-PortableEnv
$root = $script:RepoRoot

$port = [int](Get-EnvValue $envMap 'MYSQL_PORT' '3306')
$rootUser = Get-EnvValue $envMap 'MYSQL_ROOT_USER' 'root'
$rootPass = Get-EnvValue $envMap 'MYSQL_ROOT_PASSWORD' ''
$user = Get-EnvValue $envMap 'MYSQL_USER' 'mangos'
$pass = Get-EnvValue $envMap 'MYSQL_PASSWORD' 'mangos'
$realmName = Get-EnvValue $envMap 'REALM_NAME' 'TurtleWoW'
$realmAddress = Get-EnvValue $envMap 'REALM_ADDRESS' '127.0.0.1'
$worldPort = [int](Get-EnvValue $envMap 'WORLD_PORT' '8090')

$bin = Find-MariaDbBin -Root $root
if (-not $bin) { throw 'MariaDB not found. Run tools\fetch-mariadb.ps1 first.' }
$client = Get-MysqlClientPath -BinDir $bin
$sqlRoot = Join-Path $root 'sql'

function Import-SqlFile {
    param([string]$File, [string]$Database = '', [switch]$Force)
    Write-Host "  $(Split-Path -Leaf $File)"
    $argList = @("-u$rootUser", "-h127.0.0.1", "-P$port")
    if ($rootPass) { $argList = @("-u$rootUser", "-p$rootPass", "-h127.0.0.1", "-P$port") }
    if ($Force) { $argList += '--force' }
    if ($Database) { $argList += $Database }

    $quotedClient = '"' + $client + '"'
    $quotedFile = '"' + $File + '"'
    $argString = ($argList | ForEach-Object {
        if ($_ -match '\s') { '"' + $_ + '"' } else { $_ }
    }) -join ' '

    $p = Start-Process -FilePath 'cmd.exe' -ArgumentList '/c', "$quotedClient $argString < $quotedFile" -Wait -PassThru -NoNewWindow
    if ($p.ExitCode -ne 0 -and -not $Force) {
        throw "Import failed: $File (exit $($p.ExitCode))"
    }
}

Wait-MysqlReady -Client $client -User $rootUser -Password $rootPass -Port $port

Write-Host "Creating user $user (no grants yet) ..."
$createUserSql = @"
CREATE USER IF NOT EXISTS '$user'@'localhost' IDENTIFIED BY '$pass';
CREATE USER IF NOT EXISTS '$user'@'127.0.0.1' IDENTIFIED BY '$pass';
GRANT ALL PRIVILEGES ON tw_char.* TO '$user'@'localhost';
GRANT ALL PRIVILEGES ON tw_logon.* TO '$user'@'localhost';
GRANT ALL PRIVILEGES ON tw_world.* TO '$user'@'localhost';
GRANT ALL PRIVILEGES ON tw_logs.* TO '$user'@'localhost';
GRANT ALL PRIVILEGES ON tw_char.* TO '$user'@'127.0.0.1';
GRANT ALL PRIVILEGES ON tw_logon.* TO '$user'@'127.0.0.1';
GRANT ALL PRIVILEGES ON tw_world.* TO '$user'@'127.0.0.1';
GRANT ALL PRIVILEGES ON tw_logs.* TO '$user'@'127.0.0.1';
FLUSH PRIVILEGES;
"@
Invoke-Mysql -Client $client -User $rootUser -Password $rootPass -Port $port -Execute "CREATE USER IF NOT EXISTS '$user'@'localhost' IDENTIFIED BY '$pass'; CREATE USER IF NOT EXISTS '$user'@'127.0.0.1' IDENTIFIED BY '$pass'; FLUSH PRIVILEGES;"

$createSql = Join-Path $sqlRoot 'create_databases.sql'
if (-not (Test-Path $createSql)) {
    throw "Missing $createSql — run tools\sync-sql.ps1 first"
}
Write-Host "Importing create_databases.sql ..."
Import-SqlFile -File $createSql

Write-Host "Granting on tw_* ..."
Invoke-Mysql -Client $client -User $rootUser -Password $rootPass -Port $port -Execute $createUserSql

if (-not $SkipBase) {
    $baseDir = Join-Path $sqlRoot 'base'
    if (-not (Test-Path $baseDir)) { throw "Missing $baseDir" }
    $files = Get-ChildItem -LiteralPath $baseDir -Filter '*.sql' | Sort-Object Name
    Write-Host "Importing $($files.Count) base world files into tw_world ..."
    foreach ($f in $files) {
        Import-SqlFile -File $f.FullName -Database 'tw_world'
    }
}

if (-not $SkipUpdates) {
    $updatesRoot = Join-Path $sqlRoot 'database_updates'
    if (Test-Path $updatesRoot) {
        # flat *.sql plus world\ / character\ / auth\
        $updateFiles = @()
        $updateFiles += Get-ChildItem -LiteralPath $updatesRoot -Filter '*.sql' -File -ErrorAction SilentlyContinue
        foreach ($sub in @('world', 'character', 'auth')) {
            $subDir = Join-Path $updatesRoot $sub
            if (Test-Path $subDir) {
                $updateFiles += Get-ChildItem -LiteralPath $subDir -Filter '*.sql' -File -ErrorAction SilentlyContinue
            }
        }
        $updateFiles = $updateFiles | Sort-Object FullName -Unique
        Write-Host "Migrations ($($updateFiles.Count)), --force, then mark applied"
        foreach ($f in $updateFiles) {
            $db = 'tw_world'
            if ($f.Directory.Name -eq 'character') { $db = 'tw_char' }
            elseif ($f.Directory.Name -eq 'auth') { $db = 'tw_logon' }
            elseif ($f.Name -match '_character') { $db = 'tw_char' }
            elseif ($f.Name -match '_auth') { $db = 'tw_logon' }
            Import-SqlFile -File $f.FullName -Database $db -Force
            $name = [IO.Path]::GetFileNameWithoutExtension($f.Name)
            $escaped = $name.Replace("'", "''")
            try {
                Invoke-Mysql -Client $client -User $rootUser -Password $rootPass -Port $port -Database $db -Execute "INSERT IGNORE INTO migrations (Name,Hash,AppliedAt) VALUES ('$escaped','manual',NOW());"
            }
            catch {
                # no migrations table on that DB
            }
        }
    }
    else {
        Write-Warning "No database_updates — skipped"
    }
}

if (-not $SkipPlayerbots) {
    $pb = Join-Path $sqlRoot 'playerbots'
    if (Test-Path $pb) {
        Write-Host "Playerbots world SQL"
        Get-ChildItem -LiteralPath (Join-Path $pb 'world') -Filter '*.sql' -File -ErrorAction SilentlyContinue |
            Sort-Object Name |
            ForEach-Object { Import-SqlFile -File $_.FullName -Database 'tw_world' }
        $classic = Join-Path $pb 'world\classic'
        if (Test-Path $classic) {
            Get-ChildItem -LiteralPath $classic -Filter '*.sql' -File |
                Sort-Object Name |
                ForEach-Object { Import-SqlFile -File $_.FullName -Database 'tw_world' }
        }
        Write-Host "Playerbots characters SQL"
        Get-ChildItem -LiteralPath (Join-Path $pb 'characters') -Filter '*.sql' -File -ErrorAction SilentlyContinue |
            Sort-Object Name |
            ForEach-Object { Import-SqlFile -File $_.FullName -Database 'tw_char' }
    }
    else {
        Write-Warning "No sql\playerbots — mangosd will assert if bots are on"
    }
}

Write-Host "realmlist row"
Invoke-Mysql -Client $client -User $rootUser -Password $rootPass -Port $port -Execute @"
DELETE FROM tw_logon.realmlist;
INSERT INTO tw_logon.realmlist (id, name, address, port, icon, realmflags, timezone, allowedSecurityLevel, population, realmbuilds)
VALUES (1, '$realmName', '$realmAddress', $worldPort, 0, 0, 1, 0, 0, '7272');
"@

Write-Host "Import done."
