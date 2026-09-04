param(
    [switch]$Force
)

. "$PSScriptRoot\_common.ps1"

$envMap = Import-PortableEnv
$root = $script:RepoRoot
$mapsDir = Join-Path $root 'maps'
$cache = Join-Path $root 'tools\.cache'
$needed = @('dbc', 'maps', 'vmaps', 'mmaps')

function Test-MapsPresent {
    param([string]$Dir)
    foreach ($name in $needed) {
        $p = Join-Path $Dir $name
        if (-not (Test-Path $p -PathType Container)) { return $false }
        $items = Get-ChildItem -LiteralPath $p -Force -ErrorAction SilentlyContinue
        if (-not $items) { return $false }
    }
    return $true
}

function Get-DatanodesDirectUrl {
    param([string]$Key, [string]$FileCode)
    Write-Host "Resolving DataNodes direct link for $FileCode"
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    $resp = Invoke-RestMethod -Uri 'https://datanodes.to/api/file/direct_link' -Method Get -Body @{
        file_code = $FileCode
        key       = $Key
    }
    if (-not $resp -or [int]$resp.status -ne 200 -or -not $resp.result.url) {
        throw "DataNodes direct_link failed. Free page links need a wait/captcha - use the API key from your account, or host a raw .zip URL instead."
    }
    return [string]$resp.result.url
}

function Read-MapsUrlFile {
    param([string]$Path)
    if (-not (Test-Path $Path)) { return '' }
    foreach ($line in Get-Content -LiteralPath $Path) {
        $t = $line.Trim()
        if (-not $t -or $t.StartsWith('#')) { continue }
        if ($t.StartsWith('http://') -or $t.StartsWith('https://')) { return $t }
    }
    return ''
}

function Update-CachedMapsUrlFile {
    param([string]$Remote, [string]$Dest)
    if (-not $Remote) { return }
    Write-Host "Refreshing maps URL from GitHub"
    # A unique temporary avoids two setup/update invocations clobbering each
    # other's URL file while keeping the committed/cached file intact.
    $tmp = "$Dest.$([guid]::NewGuid().ToString('N')).tmp"
    try {
        $curl = Get-Command curl.exe -ErrorAction SilentlyContinue
        if ($curl) {
            & curl.exe -L --fail --silent --show-error -o $tmp $Remote
            if ($LASTEXITCODE -ne 0) { throw "curl $LASTEXITCODE" }
        }
        else {
            [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
            Invoke-WebRequest -Uri $Remote -OutFile $tmp
        }
        Move-Item -LiteralPath $tmp -Destination $Dest -Force
    }
    catch {
        Write-Warning "could not refresh maps-url.txt ($_). Using local conf\\maps-url.txt"
        if (Test-Path $tmp) { Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue }
    }
}

function Resolve-MapsZipUrl {
    $override = Get-EnvValue $envMap 'TORTOISE_WOW_MAPS_ZIP_URL' ''
    $key = Get-EnvValue $envMap 'DATANODES_API_KEY' ''
    $code = Get-EnvValue $envMap 'DATANODES_FILE_CODE' ''
    $remote = Get-EnvValue $envMap 'MAPS_URL_REMOTE' 'https://raw.githubusercontent.com/WrkX/turtleAutoInstall/main/conf/maps-url.txt'
    $cached = Join-Path $cache 'maps-url.txt'
    $committed = Join-Path $root 'conf\maps-url.txt'

    if ($code) {
        if (-not $key) {
            throw 'DATANODES_FILE_CODE is set but DATANODES_API_KEY is empty (portable.local.env).'
        }
        return Get-DatanodesDirectUrl -Key $key -FileCode $code
    }

    if ($override -match 'https?://(?:www\.)?datanodes\.to/([A-Za-z0-9]+)(?:/.*)?$') {
        $pageCode = $Matches[1]
        if ($pageCode -ne 'd' -and $pageCode -ne 'pages' -and $pageCode -ne 'api') {
            if (-not $key) {
                throw 'TORTOISE_WOW_MAPS_ZIP_URL is a DataNodes page. Set DATANODES_API_KEY, or put a direct zip URL in conf\maps-url.txt.'
            }
            return Get-DatanodesDirectUrl -Key $key -FileCode $pageCode
        }
    }

    if ($override) { return $override }

    $url = Read-MapsUrlFile $committed
    if ($url) { return $url }

    Update-CachedMapsUrlFile -Remote $remote -Dest $cached
    return Read-MapsUrlFile $cached
}

function Get-GoogleDriveFileId {
    param([string]$Url)
    if ($Url -match 'drive\.google\.com/file/d/([A-Za-z0-9_-]+)') { return $Matches[1] }
    if ($Url -match 'drive\.google\.com/open\?id=([A-Za-z0-9_-]+)') { return $Matches[1] }
    if ($Url -match '[?&]id=([A-Za-z0-9_-]+)') { return $Matches[1] }
    return $null
}

function Test-FileLooksLikeZip {
    param([string]$Path)
    if (-not (Test-Path $Path)) { return $false }
    if ((Get-Item -LiteralPath $Path).Length -lt 64) { return $false }
    $fs = [IO.File]::OpenRead($Path)
    try {
        $b0 = $fs.ReadByte()
        $b1 = $fs.ReadByte()
    }
    finally { $fs.Close() }
    return ($b0 -eq 0x50 -and $b1 -eq 0x4B)
}

function Save-HttpFile {
    param(
        [string]$Url,
        [string]$OutFile,
        [string]$UserAgent,
        [string]$CookieJar = ''
    )
    Save-DownloadFileAtomic -Url $Url -OutFile $OutFile -UserAgent $UserAgent -CookieJar $CookieJar
}

function Save-GoogleDriveFile {
    param([string]$FileId, [string]$OutFile)
    $ua = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) tortoise-wow-portable'
    $jar = Join-Path $cache 'gdrive.cookies'
    $candidateFile = "$OutFile.download"
    Write-Host "Google Drive file id $FileId"

    try {
        $candidates = @(
            "https://drive.usercontent.google.com/download?id=$FileId&export=download&confirm=t",
            "https://drive.google.com/uc?export=download&confirm=t&id=$FileId"
        )
        foreach ($u in $candidates) {
            Write-Host "Trying $u"
            Save-HttpFile -Url $u -OutFile $candidateFile -UserAgent $ua -CookieJar $jar
            if (Test-FileLooksLikeZip $candidateFile) {
                Move-Item -LiteralPath $candidateFile -Destination $OutFile -Force
                return
            }
        }

        Write-Host "Drive served a confirm page (usual for big zips) - retrying with token"
        $scan = "https://drive.google.com/uc?export=download&id=$FileId"
        Save-HttpFile -Url $scan -OutFile $candidateFile -UserAgent $ua -CookieJar $jar
        if (Test-FileLooksLikeZip $candidateFile) {
            Move-Item -LiteralPath $candidateFile -Destination $OutFile -Force
            return
        }

        $html = [IO.File]::ReadAllText($candidateFile)
        $confirm = 't'
        $uuid = ''
        if ($html -match 'name="confirm"\s+value="([^"]+)"') { $confirm = $Matches[1] }
        elseif ($html -match 'confirm=([0-9A-Za-z_-]+)') { $confirm = $Matches[1] }
        if ($html -match 'name="uuid"\s+value="([^"]+)"') { $uuid = $Matches[1] }
        $final = "https://drive.usercontent.google.com/download?id=$FileId&export=download&confirm=$confirm"
        if ($uuid) { $final += "&uuid=$uuid" }
        Write-Host "Trying $final"
        Save-HttpFile -Url $final -OutFile $candidateFile -UserAgent $ua -CookieJar $jar
        if (-not (Test-FileLooksLikeZip $candidateFile)) {
            throw 'Google Drive did not return a zip. Share as "Anyone with the link can view". Large files sometimes still hit a virus-scan page that blocks scripts - if this keeps failing, split the zip or use R2.'
        }
        Move-Item -LiteralPath $candidateFile -Destination $OutFile -Force
    }
    finally {
        Remove-Item -LiteralPath $candidateFile -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath "$candidateFile.partial" -Force -ErrorAction SilentlyContinue
    }
}

function Save-UrlToFile {
    param([string]$Url, [string]$OutFile)
    Write-Host "Downloading maps zip"
    Write-Host $Url
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $OutFile) | Out-Null
    $gId = Get-GoogleDriveFileId $Url
    if ($gId) {
        Save-GoogleDriveFile -FileId $gId -OutFile $OutFile
        return
    }
    $ua = 'tortoise-wow-portable'
    Save-HttpFile -Url $Url -OutFile $OutFile -UserAgent $ua
    if (-not (Test-FileLooksLikeZip $OutFile)) {
        Remove-Item -LiteralPath $OutFile -Force -ErrorAction SilentlyContinue
        throw 'Downloaded file is not a zip (got a webpage). Need a direct zip URL or a public Google Drive share link.'
    }
}

function Expand-ZipTo {
    param([string]$ZipPath, [string]$Dest)
    New-Item -ItemType Directory -Force -Path $Dest | Out-Null
    $tar = Get-Command tar.exe -ErrorAction SilentlyContinue
    if ($tar) {
        & tar.exe -xf $ZipPath -C $Dest
        if ($LASTEXITCODE -ne 0) { throw "tar extract failed ($LASTEXITCODE)" }
        return
    }
    Expand-Archive -LiteralPath $ZipPath -DestinationPath $Dest -Force
}

function Find-MapRoot {
    param([string]$Extract)
    $ok = {
        param($d)
        foreach ($n in $needed) {
            if (-not (Test-Path (Join-Path $d $n))) { return $false }
        }
        return $true
    }
    if (& $ok $Extract) { return $Extract }
    $nested = Join-Path $Extract 'maps'
    if (& $ok $nested) { return $nested }
    $dirs = Get-ChildItem -LiteralPath $Extract -Directory -ErrorAction SilentlyContinue
    foreach ($d in $dirs) {
        if (& $ok $d.FullName) { return $d.FullName }
        $inner = Join-Path $d.FullName 'maps'
        if (& $ok $inner) { return $inner }
    }
    return $null
}

New-Item -ItemType Directory -Force -Path $cache, $mapsDir | Out-Null

$configuredOverride = Get-EnvValue $envMap 'TORTOISE_WOW_MAPS_ZIP_URL' ''
$configuredCode = Get-EnvValue $envMap 'DATANODES_FILE_CODE' ''
$url = Resolve-MapsZipUrl
if (-not $url) {
    Write-Host 'no maps URL in conf\maps-url.txt (or GitHub copy) - skip. Put a direct https zip there, or TORTOISE_WOW_MAPS_ZIP_URL in portable.local.env.'
    return
}
$sourceIdentity = $url
if ($configuredCode) {
    # DataNodes direct links are short lived; key the cache by the stable file code.
    $sourceIdentity = "datanodes:$configuredCode"
}
elseif ($configuredOverride -match 'https?://(?:www\.)?datanodes\.to/([A-Za-z0-9]+)(?:/.*)?$') {
    $sourceIdentity = "datanodes:$($Matches[1])"
}
$urlHash = Get-StringSha256Hex -Value $sourceIdentity
$zipName = [IO.Path]::GetFileName(([Uri]$url).LocalPath)
$extension = [IO.Path]::GetExtension($zipName)
if (-not $extension -or $extension -ne '.zip') {
    $extension = '.zip'
}
$zipName = "maps-$urlHash$($extension.ToLowerInvariant())"
$zipPath = Join-Path $cache $zipName
$sourceMarker = Join-Path $root 'data\.maps-url-sha256'
$installedHash = ''
if (Test-Path $sourceMarker) {
    $installedHash = (Get-Content -LiteralPath $sourceMarker -Raw).Trim()
}

if ((Test-MapsPresent $mapsDir) -and -not $Force -and $installedHash -eq $urlHash) {
    Write-Host "Maps already at $mapsDir (source $urlHash)"
    return
}

if ($Force -or -not (Test-Path $zipPath)) {
    Save-UrlToFile -Url $url -OutFile $zipPath
}

Write-Host "Unpacking into maps\..."
$extract = Join-Path $cache 'maps-extract'
$backup = Join-Path $cache ("maps-backup-" + [guid]::NewGuid().ToString('N'))
if (Test-Path $extract) { Remove-Item -LiteralPath $extract -Recurse -Force }
New-Item -ItemType Directory -Force -Path $extract | Out-Null
$backedUp = @()
$installed = @()
$keepBackup = $false
try {
    Expand-ZipTo -ZipPath $zipPath -Dest $extract
    $src = Find-MapRoot $extract
    if (-not $src) {
        throw 'Zip unpacked but dbc/maps/vmaps/mmaps were not found. Zip those four folders (Turtle 1.18.1 / 7272).'
    }

    # Stage the replacement as a unit. Moving the old directories aside first
    # lets us restore a working map set if a later move or validation fails.
    New-Item -ItemType Directory -Force -Path $backup | Out-Null
    foreach ($name in $needed) {
        $to = Join-Path $mapsDir $name
        if (Test-Path $to) {
            Move-Item -LiteralPath $to -Destination (Join-Path $backup $name)
            $backedUp += $name
        }
    }
    foreach ($name in $needed) {
        $from = Join-Path $src $name
        $to = Join-Path $mapsDir $name
        Move-Item -LiteralPath $from -Destination $to
        $installed += $name
    }

    if (-not (Test-MapsPresent $mapsDir)) {
        throw 'Unpack finished but maps\ is still incomplete.'
    }
}
catch {
    foreach ($name in @($installed)) {
        $to = Join-Path $mapsDir $name
        if (Test-Path $to) { Remove-Item -LiteralPath $to -Recurse -Force -ErrorAction SilentlyContinue }
    }
    foreach ($name in @($backedUp)) {
        $old = Join-Path $backup $name
        $to = Join-Path $mapsDir $name
        if (Test-Path $old) {
            try {
                Move-Item -LiteralPath $old -Destination $to -Force -ErrorAction Stop
            }
            catch {
                $keepBackup = $true
                Write-Warning "could not restore maps\$name from rollback backup: $_"
            }
        }
    }
    if ($keepBackup) {
        Write-Warning "rollback was incomplete; preserving backup at $backup"
    }
    Remove-Item -LiteralPath $zipPath -Force -ErrorAction SilentlyContinue
    throw
}
finally {
    Remove-Item -LiteralPath $extract -Recurse -Force -ErrorAction SilentlyContinue
    if (-not $keepBackup) {
        Remove-Item -LiteralPath $backup -Recurse -Force -ErrorAction SilentlyContinue
    }
}

$dataDir = Join-Path $root 'data'
New-Item -ItemType Directory -Force -Path $dataDir | Out-Null
Set-Content -LiteralPath $sourceMarker -Value $urlHash -Encoding ASCII

Write-Host "OK: $mapsDir"
