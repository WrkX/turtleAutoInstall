$ErrorActionPreference = 'Stop'
$script:RepoRoot = Split-Path -Parent $PSScriptRoot

function Import-PortableEnv {
    param([string]$Root = $script:RepoRoot)

    $envFile = Join-Path $Root 'portable.env'
    $localFile = Join-Path $Root 'portable.local.env'
    $map = @{}

    foreach ($path in @($envFile, $localFile)) {
        if (-not (Test-Path $path)) { continue }
        Get-Content -LiteralPath $path | ForEach-Object {
            $line = $_.Trim()
            if (-not $line -or $line.StartsWith('#')) { return }
            $idx = $line.IndexOf('=')
            if ($idx -lt 1) { return }
            $key = $line.Substring(0, $idx).Trim()
            $val = $line.Substring($idx + 1).Trim()
            $map[$key] = $val
        }
    }
    return $map
}

function Get-EnvValue {
    param(
        [hashtable]$EnvMap,
        [string]$Name,
        [string]$Default = ''
    )
    if ($EnvMap.ContainsKey($Name) -and $null -ne $EnvMap[$Name] -and $EnvMap[$Name] -ne '') {
        return $EnvMap[$Name]
    }
    return $Default
}

function Resolve-TortoiseWowReleaseAsset {
    param(
        [hashtable]$EnvMap,
        [string]$AssetNamePattern,
        [string]$OverrideUrlKey,
        [string]$FallbackZipName,
        [string]$Kind = 'asset'
    )

    $overrideUrl = Get-EnvValue $EnvMap $OverrideUrlKey ''
    if ($overrideUrl) {
        $zipName = [IO.Path]::GetFileName(([Uri]$overrideUrl).LocalPath)
        if (-not $zipName) { $zipName = $FallbackZipName }
        return [PSCustomObject]@{
            Url     = $overrideUrl
            Name    = $zipName
            TagName = ''
        }
    }

    $repo = Get-EnvValue $EnvMap 'TORTOISE_WOW_REPO' 'WrkX/tortoise-wow'
    $release = Get-EnvValue $EnvMap 'TORTOISE_WOW_RELEASE' 'latest'
    $apiBase = "https://api.github.com/repos/$repo/releases"
    $apiUrl = if ($release -eq 'latest') { "$apiBase/latest" } else { "$apiBase/tags/$release" }

    Write-Host "Resolving $Kind from $repo release $release"
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    $headers = @{ 'User-Agent' = 'tortoise-wow-portable' }
    try {
        $releaseInfo = Invoke-RestMethod -Uri $apiUrl -Headers $headers
    }
    catch {
        throw "Could not fetch release info from $apiUrl - is the release published yet? Set $OverrideUrlKey in portable.local.env to override. $_"
    }

    $asset = $releaseInfo.assets |
        Where-Object { $_.name -like $AssetNamePattern } |
        Select-Object -First 1
    if (-not $asset) {
        throw "Release $($releaseInfo.tag_name) has no $AssetNamePattern asset"
    }

    Write-Host "Using $($releaseInfo.tag_name): $($asset.name)"
    return [PSCustomObject]@{
        Url     = $asset.browser_download_url
        Name    = $asset.name
        TagName = $releaseInfo.tag_name
    }
}

function Convert-ToIniPath {
    param([string]$Path)
    return ($Path -replace '\\', '/')
}

function Test-MysqldBinDir {
    param([string]$BinDir)
    if (-not $BinDir -or -not (Test-Path $BinDir)) { return $false }
    return (Test-Path (Join-Path $BinDir 'mysqld.exe')) -or (Test-Path (Join-Path $BinDir 'mariadbd.exe'))
}

function Find-MariaDbBin {
    param([string]$Root = $script:RepoRoot)

    $mariadbRoot = Join-Path $Root 'mariadb'
    if (-not (Test-Path $mariadbRoot)) { return $null }

    $direct = Join-Path $mariadbRoot 'bin'
    if (Test-MysqldBinDir $direct) { return $direct }

    $nested = Get-ChildItem -LiteralPath $mariadbRoot -Directory -ErrorAction SilentlyContinue |
        ForEach-Object { Join-Path $_.FullName 'bin' } |
        Where-Object { Test-MysqldBinDir $_ } |
        Select-Object -First 1

    return $nested
}

function Get-MysqldPath {
    param([string]$BinDir)
    foreach ($name in @('mariadbd.exe', 'mysqld.exe')) {
        $p = Join-Path $BinDir $name
        if (Test-Path $p) { return $p }
    }
    throw "No mariadbd.exe/mysqld.exe under $BinDir"
}

function Get-MysqlClientPath {
    param([string]$BinDir)
    foreach ($name in @('mariadb.exe', 'mysql.exe')) {
        $p = Join-Path $BinDir $name
        if (Test-Path $p) { return $p }
    }
    throw "No mariadb.exe/mysql.exe under $BinDir"
}

function Get-InstallDbPath {
    param([string]$BinDir)
    foreach ($name in @('mariadb-install-db.exe', 'mysql_install_db.exe')) {
        $p = Join-Path $BinDir $name
        if (Test-Path $p) { return $p }
    }
    return $null
}

function Test-MysqlDatadirInitialized {
    param([string]$DataDir)
    return Test-Path (Join-Path $DataDir 'mysql')
}

function Clear-MysqlDatadirForFreshInit {
    param([string]$DataDir)
    if (Test-MysqlDatadirInitialized -DataDir $DataDir) {
        throw "Refusing to wipe initialized datadir: $DataDir"
    }
    if (Test-Path $DataDir) {
        Get-ChildItem -LiteralPath $DataDir -Force | Remove-Item -Recurse -Force
    }
    else {
        New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
    }
}

function Initialize-MysqlDatadir {
    param(
        [string]$DataDir,
        [string]$BinDir,
        [string]$Mysqld,
        [string]$DefaultsFile,
        [int]$Port,
        [string]$RootPassword = ''
    )

    Clear-MysqlDatadirForFreshInit -DataDir $DataDir

    $installDb = Get-InstallDbPath -BinDir $BinDir
    if ($installDb) {
        $args = @("--datadir=$DataDir", "--port=$Port")
        if ($RootPassword) { $args += "--password=$RootPassword" }
        & $installDb @args
        if ($LASTEXITCODE -eq 0) { return }
        Write-Warning "mariadb-install-db failed ($LASTEXITCODE), falling back to --initialize-insecure"
        Clear-MysqlDatadirForFreshInit -DataDir $DataDir
    }
    else {
        Write-Host "no mariadb-install-db, falling back to --initialize-insecure"
    }

    & $Mysqld --defaults-file="$DefaultsFile" --initialize-insecure
    if ($LASTEXITCODE -ne 0) { throw "mysqld --initialize-insecure failed ($LASTEXITCODE)" }
}

function Get-MysqlClientPluginDir {
    param([string]$BinDir)
    $pluginDir = Join-Path (Split-Path -Parent $BinDir) 'lib\plugin'
    if (Test-Path $pluginDir) { return $pluginDir }
    return $null
}

function Build-MysqlClientArgs {
    param(
        [string]$BinDir = '',
        [string]$DefaultsFile = '',
        [string]$User,
        [string]$Password = '',
        [int]$Port,
        [string[]]$Extra = @()
    )

    $args = @()
    if ($DefaultsFile -and (Test-Path $DefaultsFile)) {
        $args += "--defaults-file=$DefaultsFile"
    }
    if ($BinDir) {
        $pluginDir = Get-MysqlClientPluginDir -BinDir $BinDir
        if ($pluginDir) {
            $args += "--plugin-dir=$pluginDir"
        }
    }
    if ($Password) {
        $args += @("-u$user", "-p$Password", "-h127.0.0.1", "-P$Port")
    }
    else {
        $args += @("-u$user", "-h127.0.0.1", "-P$Port")
    }
    if ($Extra) { $args += $Extra }
    return $args
}

function Test-TcpPortInUse {
    param([int]$Port)

    try {
        $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, $Port)
        $listener.Start()
        $listener.Stop()
        return $false
    }
    catch {
        return $true
    }
}

function Get-ListeningProcessForPort {
    param([int]$Port)

    try {
        $conn = @(Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue |
            Where-Object { $_.LocalAddress -in @('0.0.0.0', '127.0.0.1', '::', '::1') } |
            Select-Object -First 1)[0]
        if (-not $conn) { return $null }
        return Get-CimInstance Win32_Process -Filter "ProcessId=$($conn.OwningProcess)" -ErrorAction SilentlyContinue
    }
    catch {
        return $null
    }
}

function Assert-PortableMysqlPortAvailable {
    param(
        [int]$Port,
        [string]$MysqldPath = ''
    )

    if (-not (Test-TcpPortInUse -Port $Port)) { return }

    if ($MysqldPath) {
        $owner = Get-ListeningProcessForPort -Port $Port
        if ($owner -and $owner.ExecutablePath -and (Test-Path $MysqldPath)) {
            $ownerPath = (Resolve-Path $owner.ExecutablePath).Path
            $expectedPath = (Resolve-Path $MysqldPath).Path
            if ($ownerPath -eq $expectedPath) { return }
        }
    }

    $hint = "Port $Port is already used by another MySQL/MariaDB server."
    if ($Port -eq 3306) {
        $hint += " A local MySQL 8 install often owns 3306 (and 33060)."
    }
    $hint += " Stop that service or set MYSQL_PORT=3307 in portable.local.env, then run setup.bat again."
    throw $hint
}

function Test-MysqlReady {
    param(
        [string]$Client,
        [string]$User,
        [string]$Password,
        [int]$Port,
        [string]$BinDir = '',
        [string]$DefaultsFile = ''
    )

    $args = Build-MysqlClientArgs -BinDir $BinDir -DefaultsFile $DefaultsFile -User $User -Password $Password -Port $Port -Extra @('-e', 'SELECT 1;')
    $prevEap = $ErrorActionPreference
    $ErrorActionPreference = 'SilentlyContinue'
    try {
        & $Client @args 2>$null | Out-Null
        return ($LASTEXITCODE -eq 0)
    }
    finally {
        $ErrorActionPreference = $prevEap
    }
}

function Wait-MysqlReady {
    param(
        [string]$Client,
        [string]$User,
        [string]$Password,
        [int]$Port,
        [int]$TimeoutSec = 60,
        [string]$BinDir = '',
        [string]$DefaultsFile = ''
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        if (Test-MysqlReady -Client $Client -User $User -Password $Password -Port $Port -BinDir $BinDir -DefaultsFile $DefaultsFile) {
            return
        }
        Start-Sleep -Seconds 1
    }
    throw "MySQL did not become ready on port $Port within ${TimeoutSec}s"
}

function Invoke-Mysql {
    param(
        [string]$Client,
        [string]$User,
        [string]$Password,
        [int]$Port,
        [string]$Database = '',
        [string]$Execute = '',
        [string]$InputFile = '',
        [switch]$Force,
        [string]$BinDir = '',
        [string]$DefaultsFile = ''
    )
    $extra = @()
    if ($Execute) { $extra += @('-e', $Execute) }
    $args = Build-MysqlClientArgs -BinDir $BinDir -DefaultsFile $DefaultsFile -User $User -Password $Password -Port $Port -Extra $extra
    if ($Force) { $args += '--force' }
    if ($Database) { $args += $Database }
    if ($Execute) {
        & $Client @args
    }
    elseif ($InputFile) {
        Get-Content -LiteralPath $InputFile -Raw | & $Client @args
    }
    else {
        throw 'Invoke-Mysql requires -Execute or -InputFile'
    }
    if ($LASTEXITCODE -ne 0 -and -not $Force) {
        throw "mysql failed (exit $LASTEXITCODE)"
    }
}

function Get-FileSha1Hex {
    param([string]$Path)
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA1).Hash.ToUpperInvariant()
}

function Set-MigrationApplied {
    param(
        [string]$Client,
        [string]$BinDir,
        [string]$DefaultsFile,
        [string]$RootUser,
        [string]$RootPass,
        [int]$Port,
        [string]$Database,
        [string]$MigrationFile
    )

    $name = [IO.Path]::GetFileNameWithoutExtension($MigrationFile)
    $hash = Get-FileSha1Hex -Path $MigrationFile
    $escapedName = $name.Replace("'", "''")
    $escapedHash = $hash.Replace("'", "''")
    Invoke-Mysql -Client $Client -User $RootUser -Password $RootPass -Port $Port -BinDir $BinDir -DefaultsFile $DefaultsFile -Database $Database -Execute "INSERT INTO migrations (Name,Hash,AppliedAt) VALUES ('$escapedName','$escapedHash',NOW()) ON DUPLICATE KEY UPDATE Hash='$escapedHash', AppliedAt=NOW();"
}

function Write-FromTemplate {
    param(
        [string]$TemplatePath,
        [string]$OutPath,
        [hashtable]$Replacements
    )
    $text = Get-Content -LiteralPath $TemplatePath -Raw
    foreach ($key in $Replacements.Keys) {
        $text = $text.Replace($key, [string]$Replacements[$key])
    }
    $dir = Split-Path -Parent $OutPath
    if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir | Out-Null }
    Set-Content -LiteralPath $OutPath -Value $text -NoNewline -Encoding ASCII
}

function Patch-ConfDatabaseLines {
    param(
        [string]$Path,
        [string]$DbHost,
        [int]$Port,
        [string]$User,
        [string]$Password
    )
    if (-not (Test-Path $Path)) { throw "Missing conf: $Path" }
    $content = Get-Content -LiteralPath $Path -Raw
    $info = "`"$DbHost;$Port;$User;$Password"
    $replacements = @{
        'LoginDatabase\.Info\s*=\s*".*"'      = "LoginDatabase.Info = $info;tw_logon`""
        'LoginDatabaseInfo\s*=\s*".*"'        = "LoginDatabaseInfo = $info;tw_logon`""
        'WorldDatabase\.Info\s*=\s*".*"'      = "WorldDatabase.Info = $info;tw_world`""
        'CharacterDatabase\.Info\s*=\s*".*"'  = "CharacterDatabase.Info = $info;tw_char`""
        'LogsDatabase\.Info\s*=\s*".*"'       = "LogsDatabase.Info = $info;tw_logs`""
    }
    foreach ($pattern in $replacements.Keys) {
        $content = [regex]::Replace($content, $pattern, $replacements[$pattern])
    }
    Set-Content -LiteralPath $Path -Value $content -NoNewline -Encoding ASCII
}

function Set-ConfValue {
    param(
        [string]$Path,
        [string]$Key,
        [string]$Value
    )
    $content = Get-Content -LiteralPath $Path -Raw
    $pattern = "(?m)^(\s*)" + [regex]::Escape($Key) + "\s*=\s*.*$"
    if ([regex]::IsMatch($content, $pattern)) {
        $content = [regex]::Replace($content, $pattern, "`$1$Key = $Value")
    }
    else {
        $content = $content.TrimEnd() + "`r`n$Key = $Value`r`n"
    }
    Set-Content -LiteralPath $Path -Value $content -NoNewline -Encoding ASCII
}
