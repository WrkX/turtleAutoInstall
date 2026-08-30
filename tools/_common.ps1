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

function Get-StringSha256Hex {
    param([string]$Value)

    $sha = [Security.Cryptography.SHA256]::Create()
    try {
        $bytes = [Text.Encoding]::UTF8.GetBytes($Value)
        return ([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant()
    }
    finally {
        $sha.Dispose()
    }
}

function Save-DownloadFileAtomic {
    param(
        [Parameter(Mandatory = $true)][string]$Url,
        [Parameter(Mandatory = $true)][string]$OutFile,
        [string]$UserAgent = '',
        [string]$CookieJar = ''
    )

    $parent = Split-Path -Parent $OutFile
    if ($parent) { New-Item -ItemType Directory -Force -Path $parent | Out-Null }
    $partial = "$OutFile.partial"
    if (Test-Path $partial) {
        Remove-Item -LiteralPath $partial -Force -ErrorAction SilentlyContinue
    }

    try {
        $curl = Get-Command curl.exe -ErrorAction SilentlyContinue
        if ($curl) {
            # Do not use --silent: curl's progress meter is useful for large assets.
            $curlArgs = @('-L', '--fail', '--retry', '3', '-o', $partial)
            if ($UserAgent) { $curlArgs += @('-A', $UserAgent) }
            if ($CookieJar) { $curlArgs = @('-c', $CookieJar, '-b', $CookieJar) + $curlArgs }
            $curlArgs += $Url
            & curl.exe @curlArgs
            if ($LASTEXITCODE -ne 0) { throw "curl download failed ($LASTEXITCODE)" }
        }
        else {
            [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
            $invokeArgs = @{ Uri = $Url; OutFile = $partial; UseBasicParsing = $true }
            if ($UserAgent) { $invokeArgs['UserAgent'] = $UserAgent }
            Invoke-WebRequest @invokeArgs
        }

        if (-not (Test-Path $partial)) { throw 'download completed without creating an output file' }
        # The rename keeps an existing complete file until the new download is ready.
        Move-Item -LiteralPath $partial -Destination $OutFile -Force
    }
    catch {
        if (Test-Path $partial) {
            Remove-Item -LiteralPath $partial -Force -ErrorAction SilentlyContinue
        }
        throw
    }
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

function ConvertTo-SqlLiteral {
    param([AllowNull()][string]$Value)
    if ($null -eq $Value) { return 'NULL' }
    if ($Value.IndexOf([char]0) -ge 0) { throw 'SQL values may not contain NUL bytes.' }
    # MariaDB accepts doubled quotes; doubled backslashes prevent a value from
    # accidentally turning the following character into an escape sequence.
    return "'" + $Value.Replace('\', '\\').Replace("'", "''") + "'"
}

function ConvertTo-SqlIdentifier {
    param([Parameter(Mandatory)][string]$Value)
    if ($Value.IndexOf([char]0) -ge 0) { throw 'SQL identifiers may not contain NUL bytes.' }
    return '`' + $Value.Replace('`', '``') + '`'
}

function New-MysqlDefaultsExtraFile {
    param(
        [string]$User = 'root',
        [string]$Password = '',
        [string]$ClientHost = '127.0.0.1',
        [int]$Port = 3307
    )

    $dir = Join-Path $script:RepoRoot 'data\mysql-tmp'
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    $path = Join-Path $dir ('.mysql-client-' + [guid]::NewGuid().ToString('N') + '.cnf')
    # Double quotes and backslashes are understood by the MariaDB option file
    # parser. Keep the secret in this short-lived file rather than argv.
    $quotedPassword = $Password.Replace('\', '\\').Replace('"', '\"')
    $quotedUser = $User.Replace('\', '\\').Replace('"', '\"')
    $quotedHost = $ClientHost.Replace('\', '\\').Replace('"', '\"')
    $text = "[client]`r`nuser=`"$quotedUser`"`r`npassword=`"$quotedPassword`"`r`nhost=`"$quotedHost`"`r`nport=$Port`r`n"
    Set-Content -LiteralPath $path -Value $text -NoNewline -Encoding ASCII

    # On Windows, make the file private to the current account. Set-Acl is not
    # available in every PowerShell host, so cleanup remains the final guard.
    try {
        $acl = Get-Acl -LiteralPath $path
        $acl.SetAccessRuleProtection($true, $false)
        $identity = [Security.Principal.WindowsIdentity]::GetCurrent().Name
        $rule = New-Object -TypeName Security.AccessControl.FileSystemAccessRule -ArgumentList @(
            $identity, 'FullControl', 'Allow')
        $acl.SetAccessRule($rule)
        Set-Acl -LiteralPath $path -AclObject $acl
    }
    catch {
        # Non-Windows PowerShell or restricted ACL providers: the file is still
        # deleted in the caller's finally block.
    }
    return $path
}

function Remove-MysqlDefaultsExtraFile {
    param([string[]]$ArgumentList)
    foreach ($arg in @($ArgumentList)) {
        if ($arg -match '^--defaults-extra-file=(.+)$') {
            Remove-Item -LiteralPath $Matches[1] -Force -ErrorAction SilentlyContinue
        }
    }
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

    $initialized = $false
    $installDb = Get-InstallDbPath -BinDir $BinDir
    if ($installDb) {
        $args = @("--datadir=$DataDir", "--port=$Port")
        # The Windows install utility only accepts --password on argv. Create
        # an empty local root account, then set a requested password over stdin
        # after the server starts instead.
        & $installDb @args
        $initExit = $LASTEXITCODE
        if ($initExit -eq 0) {
            $initialized = $true
        }
        else {
            Write-Warning "mariadb-install-db failed ($initExit), falling back to --initialize-insecure"
            Clear-MysqlDatadirForFreshInit -DataDir $DataDir
        }
    }
    else {
        Write-Host "no mariadb-install-db, falling back to --initialize-insecure"
    }

    if (-not $initialized) {
        & $Mysqld --defaults-file="$DefaultsFile" --initialize-insecure
        if ($LASTEXITCODE -ne 0) { throw "mysqld --initialize-insecure failed ($LASTEXITCODE)" }
    }

    if ($RootPassword) {
        $client = Get-MysqlClientPath -BinDir $BinDir
        Write-Host 'setting the initial root password'
        $bootstrap = Start-Process -FilePath $Mysqld -ArgumentList "--defaults-file=$DefaultsFile" -PassThru -WindowStyle Minimized
        try {
            Wait-MysqlReady -Client $client -User 'root' -Password '' -Port $Port -BinDir $BinDir -DefaultsFile $DefaultsFile -TimeoutSec 90
            $passwordSql = ConvertTo-SqlLiteral $RootPassword
            Invoke-Mysql -Client $client -User 'root' -Password '' -Port $Port -BinDir $BinDir -DefaultsFile $DefaultsFile -Execute @"
ALTER USER IF EXISTS 'root'@'localhost' IDENTIFIED BY $passwordSql;
ALTER USER IF EXISTS 'root'@'127.0.0.1' IDENTIFIED BY $passwordSql;
FLUSH PRIVILEGES;
"@
            if (-not (Test-MysqlReady -Client $client -User 'root' -Password $RootPassword -Port $Port -BinDir $BinDir -DefaultsFile $DefaultsFile)) {
                throw 'Root password was set but could not be verified over the portable MySQL connection.'
            }
        }
        finally {
            if (-not $bootstrap.HasExited) {
                try {
                    Invoke-Mysql -Client $client -User 'root' -Password $RootPassword -Port $Port -BinDir $BinDir -DefaultsFile $DefaultsFile -Execute 'SHUTDOWN;'
                    $bootstrap.WaitForExit(30000) | Out-Null
                }
                catch {
                    Stop-Process -Id $bootstrap.Id -Force -ErrorAction SilentlyContinue
                }
            }
        }
    }
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
    if ($Password) {
        $credentialFile = New-MysqlDefaultsExtraFile -User $User -Password $Password -Port $Port
        # MariaDB requires option-file switches to be the first argument. The
        # generated file contains every client setting we need, so do not mix
        # --defaults-file and --defaults-extra-file.
        $args += "--defaults-extra-file=$credentialFile"
    }
    elseif ($DefaultsFile -and (Test-Path $DefaultsFile)) {
        $args += "--defaults-file=$DefaultsFile"
    }
    if ($BinDir) {
        $pluginDir = Get-MysqlClientPluginDir -BinDir $BinDir
        if ($pluginDir) {
            $args += "--plugin-dir=$pluginDir"
        }
    }
    $args += @("-u$user", "-h127.0.0.1", "-P$Port")
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

function Resolve-ProcessExecutablePath {
    param([Parameter(Mandatory)][string]$Path)
    if (-not (Test-Path -LiteralPath $Path)) { return $null }
    return [IO.Path]::GetFullPath((Resolve-Path -LiteralPath $Path).Path).TrimEnd('\').ToLowerInvariant()
}

function Get-PortableProcessesByPath {
    param([Parameter(Mandatory)][string]$ExecutablePath)
    $expected = Resolve-ProcessExecutablePath -Path $ExecutablePath
    if (-not $expected) { return @() }
    $name = Split-Path -Leaf $ExecutablePath
    $found = @()
    try {
        $found = @(Get-CimInstance Win32_Process -Filter "Name='$name'" -ErrorAction SilentlyContinue |
            Where-Object {
                $_.ExecutablePath -and
                (Resolve-ProcessExecutablePath -Path ([string]$_.ExecutablePath)) -eq $expected
            })
    }
    catch { $found = @() }
    return $found
}

function Get-PortablePidFilePath {
    param([Parameter(Mandatory)][string]$ProcessName)
    return Join-Path $script:RepoRoot ("data\$ProcessName.pid")
}

function Read-PortablePid {
    param([Parameter(Mandatory)][string]$ProcessName)
    $file = Get-PortablePidFilePath -ProcessName $ProcessName
    if (-not (Test-Path -LiteralPath $file)) { return $null }
    $content = Get-Content -LiteralPath $file -Raw -ErrorAction SilentlyContinue
    if ($null -eq $content) { return $null }
    $raw = $content.Trim()
    $parsedPid = 0
    if ([int]::TryParse($raw, [ref]$parsedPid) -and $parsedPid -gt 0) { return $parsedPid }
    return $null
}

function Write-PortablePid {
    param([Parameter(Mandatory)][string]$ProcessName, [Parameter(Mandatory)][int]$ProcessId)
    $file = Get-PortablePidFilePath -ProcessName $ProcessName
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $file) | Out-Null
    Set-Content -LiteralPath $file -Value $ProcessId -Encoding ASCII
}

function Remove-PortablePid {
    param([Parameter(Mandatory)][string]$ProcessName)
    Remove-Item -LiteralPath (Get-PortablePidFilePath -ProcessName $ProcessName) -Force -ErrorAction SilentlyContinue
}

function Test-PortableProcessId {
    param([int]$ProcessId, [Parameter(Mandatory)][string]$ExecutablePath)
    if ($ProcessId -le 0) { return $false }
    $expected = Resolve-ProcessExecutablePath -Path $ExecutablePath
    if (-not $expected) { return $false }
    try {
        $process = Get-CimInstance Win32_Process -Filter "ProcessId=$ProcessId" -ErrorAction SilentlyContinue
        return ($process -and $process.ExecutablePath -and
            (Resolve-ProcessExecutablePath -Path ([string]$process.ExecutablePath)) -eq $expected)
    }
    catch { return $false }
}

function Assert-PortableMapsPresent {
    param([string]$MapsDir = (Join-Path $script:RepoRoot 'maps'))
    foreach ($name in @('dbc', 'maps', 'vmaps', 'mmaps')) {
        $path = Join-Path $MapsDir $name
        if (-not (Test-Path -LiteralPath $path -PathType Container)) {
            throw "Missing maps\$name. Run setup without -SkipDownload or provide the four client-data directories."
        }
        $items = Get-ChildItem -LiteralPath $path -Force -ErrorAction SilentlyContinue
        if (-not $items) {
            throw "maps\$name is empty. Setup requires dbc, maps, vmaps, and mmaps to contain data."
        }
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

    $args = Build-MysqlClientArgs -BinDir $BinDir -DefaultsFile $DefaultsFile -User $User -Password $Password -Port $Port
    $prevEap = $ErrorActionPreference
    $ErrorActionPreference = 'SilentlyContinue'
    try {
        'SELECT 1;' | & $Client @args 2>$null | Out-Null
        return ($LASTEXITCODE -eq 0)
    }
    finally {
        Remove-MysqlDefaultsExtraFile -ArgumentList $args
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
        [switch]$CaptureOutput,
        [string]$BinDir = '',
        [string]$DefaultsFile = ''
    )
    $extra = @()
    $args = Build-MysqlClientArgs -BinDir $BinDir -DefaultsFile $DefaultsFile -User $User -Password $Password -Port $Port -Extra $extra
    if ($Force) { $args += '--force' }
    if ($Database) { $args += $Database }
    try {
        if ($Execute) {
            # SQL can contain passwords and realm settings; stdin keeps it out
            # of the process command line as well.
            $output = @($Execute | & $Client @args)
        }
        elseif ($InputFile) {
            $output = @(Get-Content -LiteralPath $InputFile -Raw | & $Client @args)
        }
        else {
            throw 'Invoke-Mysql requires -Execute or -InputFile'
        }
        $exitCode = $LASTEXITCODE
        if ($output) { $output }
        if ($exitCode -ne 0 -and -not $Force) {
            throw "mysql failed (exit $exitCode)"
        }
    }
    finally {
        Remove-MysqlDefaultsExtraFile -ArgumentList $args
    }
}

function Ensure-PortableDatabaseUser {
    param(
        [string]$Client,
        [string]$BinDir,
        [string]$DefaultsFile,
        [string]$RootUser,
        [string]$RootPass,
        [int]$Port,
        [Parameter(Mandatory = $true)][string]$User,
        [string]$Password = ''
    )

    # setup.ps1 runs this on every config apply. CREATE USER IF NOT EXISTS is
    # intentionally not enough: changing MYSQL_PASSWORD otherwise rewrites the
    # server confs while leaving the database account on its old password.
    $userSql = ConvertTo-SqlLiteral $User
    $passSql = ConvertTo-SqlLiteral $Password
    $sql = @"
CREATE USER IF NOT EXISTS $userSql@'localhost' IDENTIFIED BY $passSql;
CREATE USER IF NOT EXISTS $userSql@'127.0.0.1' IDENTIFIED BY $passSql;
ALTER USER $userSql@'localhost' IDENTIFIED BY $passSql;
ALTER USER $userSql@'127.0.0.1' IDENTIFIED BY $passSql;
GRANT ALL PRIVILEGES ON tw_char.* TO $userSql@'localhost';
GRANT ALL PRIVILEGES ON tw_logon.* TO $userSql@'localhost';
GRANT ALL PRIVILEGES ON tw_world.* TO $userSql@'localhost';
GRANT ALL PRIVILEGES ON tw_logs.* TO $userSql@'localhost';
GRANT ALL PRIVILEGES ON tw_char.* TO $userSql@'127.0.0.1';
GRANT ALL PRIVILEGES ON tw_logon.* TO $userSql@'127.0.0.1';
GRANT ALL PRIVILEGES ON tw_world.* TO $userSql@'127.0.0.1';
GRANT ALL PRIVILEGES ON tw_logs.* TO $userSql@'127.0.0.1';
FLUSH PRIVILEGES;
"@
    Invoke-Mysql -Client $Client -User $RootUser -Password $RootPass -Port $Port -BinDir $BinDir -DefaultsFile $DefaultsFile -Execute $sql
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
    foreach ($part in @(@{ Name = 'MYSQL_USER'; Value = $User }, @{ Name = 'MYSQL_PASSWORD'; Value = $Password })) {
        if ($part.Value -match '[;"\r\n]') {
            throw "$($part.Name) cannot contain semicolons, double quotes, or line breaks because server database settings are semicolon-delimited."
        }
    }
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
