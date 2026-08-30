param(
    [Parameter(Mandatory)][string]$Username,
    [int]$GMLevel = 0
)

. "$PSScriptRoot\_common.ps1"

$ShaPassHash = $null
if ($Username -notmatch '^[A-Za-z0-9]{2,16}$') {
    throw 'username must be 2-16 letters or digits'
}
$hashFromEnv = [Environment]::GetEnvironmentVariable('TORTOISE_WOW_ACCOUNT_SHA_PASS_HASH')
if ($hashFromEnv) {
    # The hash is accepted only through the short-lived environment handoff;
    # a -ShaPassHash argument would be visible in the PowerShell process argv.
    $ShaPassHash = $hashFromEnv
    # Do not let the hash propagate to mysql.exe or any later child process.
    [Environment]::SetEnvironmentVariable('TORTOISE_WOW_ACCOUNT_SHA_PASS_HASH', $null, 'Process')
}
if (-not $ShaPassHash) {
    $securePassword = Read-Host 'Password (1-16 characters)' -AsSecureString
    $passwordPtr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($securePassword)
    try {
        $plainPassword = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($passwordPtr)
        if ($plainPassword.Length -lt 1 -or $plainPassword.Length -gt 16) {
            throw 'password must be 1-16 characters'
        }
        $hashInput = $Username.ToUpperInvariant() + ':' + $plainPassword.ToUpperInvariant()
        $sha1 = [Security.Cryptography.SHA1]::Create()
        try {
            $bytes = [Text.Encoding]::UTF8.GetBytes($hashInput)
            $ShaPassHash = ([BitConverter]::ToString($sha1.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant()
        }
        finally { $sha1.Dispose() }
    }
    finally {
        [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($passwordPtr)
        $plainPassword = $null
    }
}
if ($ShaPassHash -notmatch '^[0-9A-Fa-f]{40}$') {
    throw 'sha_pass_hash must be a 40-char hex SHA1'
}
if ($GMLevel -lt 0 -or $GMLevel -gt 3) {
    throw 'GM level must be 0-3'
}

$envMap = Import-PortableEnv
$root = $script:RepoRoot
$port = [int](Get-EnvValue $envMap 'MYSQL_PORT' '3307')
$rootUser = Get-EnvValue $envMap 'MYSQL_ROOT_USER' 'root'
$rootPass = Get-EnvValue $envMap 'MYSQL_ROOT_PASSWORD' ''

$bin = Find-MariaDbBin -Root $root
if (-not $bin) { throw 'MariaDB not found. Run Full setup first.' }
$client = Get-MysqlClientPath -BinDir $bin
$myIni = Join-Path $root 'conf\my.ini'

function Invoke-MysqlQuiet {
    param([string]$Query)
    $out = Invoke-Mysql -Client $client -User $rootUser -Password $rootPass -Port $port -BinDir $bin -DefaultsFile $myIni -Execute $Query -CaptureOutput
    return (@($out) | Out-String).Trim()
}

Wait-MysqlReady -Client $client -User $rootUser -Password $rootPass -Port $port -BinDir $bin -DefaultsFile $myIni -TimeoutSec 30

$user = $Username.ToUpperInvariant()
$hash = $ShaPassHash.ToLowerInvariant()
$userSql = ConvertTo-SqlLiteral $user
$hashSql = ConvertTo-SqlLiteral $hash

$colsRaw = Invoke-MysqlQuiet "SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='tw_logon' AND TABLE_NAME='account'"
if (-not $colsRaw) {
    throw 'tw_logon.account not found — run Full setup first'
}
$colSet = @{}
foreach ($line in ($colsRaw -split '\r?\n')) {
    $n = $line.Trim().ToLowerInvariant()
    if ($n) { $colSet[$n] = $true }
}
if (-not $colSet.ContainsKey('username') -or -not $colSet.ContainsKey('sha_pass_hash')) {
    throw 'tw_logon.account is missing username/sha_pass_hash'
}

$existing = Invoke-MysqlQuiet "SELECT id FROM tw_logon.account WHERE username=$userSql LIMIT 1"
$action = 'created'
if ($existing -match '^\d+$') {
    $action = 'updated'
    $set = "sha_pass_hash=$hashSql"
    if ($colSet.ContainsKey('v')) { $set += ", v=''" }
    if ($colSet.ContainsKey('s')) { $set += ", s=''" }
    if ($colSet.ContainsKey('sessionkey')) { $set += ", sessionkey=''" }
    if ($colSet.ContainsKey('gmlevel')) { $set += ", gmlevel=$GMLevel" }
    Invoke-MysqlQuiet "UPDATE tw_logon.account SET $set WHERE id=$existing" | Out-Null
    $id = $existing
}
else {
    $colNames = @('username', 'sha_pass_hash')
    $values = @($userSql, $hashSql)
    if ($colSet.ContainsKey('gmlevel')) {
        $colNames += 'gmlevel'
        $values += "$GMLevel"
    }
    if ($colSet.ContainsKey('expansion')) {
        $colNames += 'expansion'
        $values += '0'
    }
    Invoke-MysqlQuiet "INSERT INTO tw_logon.account ($($colNames -join ', ')) VALUES ($($values -join ', '))" | Out-Null
    $id = Invoke-MysqlQuiet "SELECT id FROM tw_logon.account WHERE username=$userSql LIMIT 1"
}

$hasAccess = Invoke-MysqlQuiet "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='tw_logon' AND TABLE_NAME='account_access'"
if ($hasAccess -eq '1' -and $id -match '^\d+$') {
    Invoke-MysqlQuiet "DELETE FROM tw_logon.account_access WHERE id=$id" | Out-Null
    if ($GMLevel -gt 0) {
        Invoke-MysqlQuiet "INSERT INTO tw_logon.account_access (id, gmlevel, RealmID) VALUES ($id, $GMLevel, -1)" | Out-Null
    }
}

Write-Host "OK: account $user $action (id $id, gm $GMLevel)"
Write-Host "set realmlist $(Get-EnvValue $envMap 'REALM_ADDRESS' '127.0.0.1')"
