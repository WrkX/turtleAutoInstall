. "$PSScriptRoot\_common.ps1"

$envMap = Import-PortableEnv
$root = $script:RepoRoot
$port = [int](Get-EnvValue $envMap 'MYSQL_PORT' '3307')
$rootUser = Get-EnvValue $envMap 'MYSQL_ROOT_USER' 'root'
$rootPass = Get-EnvValue $envMap 'MYSQL_ROOT_PASSWORD' ''
$realmName = Get-EnvValue $envMap 'REALM_NAME' 'TurtleWoW'
$realmAddress = Get-EnvValue $envMap 'REALM_ADDRESS' '127.0.0.1'
$worldPort = [int](Get-EnvValue $envMap 'WORLD_PORT' '8090')

$bin = Find-MariaDbBin -Root $root
if (-not $bin) { throw 'MariaDB not found. Run setup.bat first.' }
$client = Get-MysqlClientPath -BinDir $bin
$myIni = Join-Path $root 'conf\my.ini'
if (-not (Test-Path -LiteralPath $myIni)) { throw "no $myIni - run setup.bat first." }

Wait-MysqlReady -Client $client -User $rootUser -Password $rootPass -Port $port -BinDir $bin -DefaultsFile $myIni
$nameSql = ConvertTo-SqlLiteral $realmName
$addressSql = ConvertTo-SqlLiteral $realmAddress
$sql = @"
INSERT INTO tw_logon.realmlist (id, name, address, port, icon, realmflags, timezone, allowedSecurityLevel, population, realmbuilds)
VALUES (1, $nameSql, $addressSql, $worldPort, 0, 0, 1, 0, 0, '7272')
ON DUPLICATE KEY UPDATE name=VALUES(name), address=VALUES(address), port=VALUES(port);
"@

Write-Host "syncing realmlist ($realmName -> $realmAddress`:$worldPort)"
Invoke-Mysql -Client $client -User $rootUser -Password $rootPass -Port $port -BinDir $bin -DefaultsFile $myIni -Execute $sql
