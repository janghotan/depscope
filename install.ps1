# DepScope CLI 安装脚本（Windows，PowerShell 5.1 及以上）。
#
# 用法：
#   irm https://raw.githubusercontent.com/janghotan/depscope/main/install.ps1 | iex
#
# 可通过环境变量配置：
#   $env:DEPSCOPE_VERSION  = "v0.1.0"   指定版本，默认取最新已发布版本
#   $env:DEPSCOPE_INSTALL_DIR           指定安装目录
#
# 安装流程：确定平台 → 下载 checksums.txt 与对应 ZIP → 校验 SHA-256 →
# 解压 → 安装 depscope.exe。校验失败时不会解压或执行任何下载内容。
# 默认安装到 %LOCALAPPDATA%\Programs\DepScope\bin，不需要管理员权限。

[CmdletBinding()]
param(
    [string]$Version = $env:DEPSCOPE_VERSION,
    [string]$InstallDir = $env:DEPSCOPE_INSTALL_DIR,
    [switch]$Help
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version 2.0

$Repository = 'janghotan/depscope'
$BinaryName = 'depscope'

# 发布与 API 地址默认指向官方 GitHub；环境变量仅用于测试本地模拟服务器，
# 正常安装无需设置，也不会削弱 HTTPS 强制与证书校验。
$ReleaseBase = "https://github.com/$Repository/releases"
$ApiBase = "https://api.github.com/repos/$Repository"
if ($env:DEPSCOPE_RELEASE_BASE) { $ReleaseBase = $env:DEPSCOPE_RELEASE_BASE.TrimEnd('/') }
if ($env:DEPSCOPE_API_BASE) { $ApiBase = $env:DEPSCOPE_API_BASE.TrimEnd('/') }

function Write-Info {
    param([string]$Message)
    Write-Host "depscope: $Message"
}

function Stop-WithError {
    param([string]$Message)
    Write-Error "depscope: error: $Message"
    exit 1
}

function Show-Usage {
    @'
Install the DepScope CLI from GitHub Releases.

Usage:
  install.ps1 [-Version <tag>] [-InstallDir <path>]

Options:
  -Version <tag>       Release tag to install, for example v0.1.0
                       (default: the latest published CLI release)
  -InstallDir <path>   Install directory
                       (default: $env:LOCALAPPDATA\Programs\DepScope\bin)
  -Help                Show this help

Supported platforms: Windows on amd64 and arm64.

The archive is verified against the release checksums.txt before it is
extracted. Never run this script with administrator privileges; it installs
into the current user's profile.
'@
}

if ($Help) {
    Show-Usage
    exit 0
}

# 下载只允许 HTTPS；不提供关闭证书校验的开关，避免此处被削弱。
function Invoke-Download {
    param([string]$Uri, [string]$OutFile)

    if ($Uri -notmatch '^https://') {
        Stop-WithError "refusing to download over a non-HTTPS URL: $Uri"
    }
    try {
        # PowerShell 5.1 默认可能使用 TLS 1.0，这里显式启用 TLS 1.2。
        [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.SecurityProtocolType]::Tls12
        Invoke-WebRequest -Uri $Uri -OutFile $OutFile -UseBasicParsing -ErrorAction Stop
    }
    catch {
        Stop-WithError "could not download $Uri : $($_.Exception.Message)"
    }
}

# 解析最新已发布（非预发布）版本标签。
function Resolve-LatestTag {
    $apiUrl = "$ApiBase/releases/latest"
    try {
        [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.SecurityProtocolType]::Tls12
        $response = Invoke-WebRequest -Uri $apiUrl -UseBasicParsing -ErrorAction Stop `
            -Headers @{ 'Accept' = 'application/vnd.github+json' }
        # Windows PowerShell 5.1 返回字符串，PowerShell 7 返回字节数组；
        # 统一转成字符串后再解析，否则 ConvertFrom-Json 会把字节当成数组。
        $rawContent = $response.Content
        if ($rawContent -is [byte[]]) {
            $rawContent = [System.Text.Encoding]::UTF8.GetString($rawContent)
        }
        $payload = $rawContent | ConvertFrom-Json
    }
    catch {
        Stop-WithError "could not resolve the latest release from $apiUrl; pass -Version <tag> instead. $($_.Exception.Message)"
    }
    # StrictMode 下访问不存在的属性会抛出异常，因此先检查属性是否存在。
    $tagProperty = $payload.PSObject.Properties['tag_name']
    if ($null -eq $tagProperty -or -not $tagProperty.Value) {
        Stop-WithError 'the latest release response did not contain a tag_name field'
    }
    return [string]$tagProperty.Value
}

function Get-ReleaseArch {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        'AMD64' { return 'amd64' }
        'ARM64' { return 'arm64' }
        default {
            Stop-WithError "unsupported processor architecture '$($env:PROCESSOR_ARCHITECTURE)'; DepScope provides amd64 and arm64 builds for Windows"
        }
    }
}

# 读取 checksums.txt 并返回目标文件的期望摘要；缺失时返回 $null。
function Get-ExpectedChecksum {
    param([string]$ChecksumsPath, [string]$FileName)

    foreach ($line in Get-Content -LiteralPath $ChecksumsPath) {
        $trimmed = $line.Trim()
        if ($trimmed -eq '') { continue }
        $parts = $trimmed -split '\s+', 2
        if ($parts.Count -ne 2) { continue }
        $name = $parts[1].Trim()
        if ($name.StartsWith('*')) { $name = $name.Substring(1) }
        if ($name -eq $FileName) { return $parts[0].ToLowerInvariant() }
    }
    return $null
}

function Test-OnPath {
    param([string]$Directory)
    $normalized = $Directory.TrimEnd('\')
    foreach ($entry in ([string]$env:PATH -split ';')) {
        if ($entry.TrimEnd('\') -ieq $normalized) { return $true }
    }
    return $false
}

if (-not $InstallDir) {
    if (-not $env:LOCALAPPDATA) {
        Stop-WithError 'LOCALAPPDATA is not set; pass -InstallDir <path> explicitly'
    }
    $InstallDir = Join-Path $env:LOCALAPPDATA 'Programs\DepScope\bin'
}

$arch = Get-ReleaseArch

if (-not $Version) {
    $Version = Resolve-LatestTag
}

if ($Version -notmatch '^[A-Za-z0-9._-]+$') {
    Stop-WithError "invalid version tag '$Version'"
}

# GoReleaser 去掉标签前缀 v 作为归档版本名。
$assetVersion = $Version -replace '^v', ''
$archiveName = "depscope_cli_${assetVersion}_windows_${arch}.zip"
$releaseBase = "$ReleaseBase/download/$Version"

Write-Info "installing DepScope $Version for windows/$arch"

$workRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("depscope-install-" + [System.Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $workRoot -Force | Out-Null

try {
    $checksumsPath = Join-Path $workRoot 'checksums.txt'
    $archivePath = Join-Path $workRoot $archiveName

    # 1. 下载校验清单并确认目标归档已登记。
    Invoke-Download -Uri "$releaseBase/checksums.txt" -OutFile $checksumsPath
    $expected = Get-ExpectedChecksum -ChecksumsPath $checksumsPath -FileName $archiveName
    if (-not $expected) {
        Stop-WithError "checksums.txt for $Version does not list $archiveName; this release has no CLI binary for windows/$arch"
    }

    # 2. 下载归档。
    Invoke-Download -Uri "$releaseBase/$archiveName" -OutFile $archivePath

    # 3. 在任何解压或执行之前校验 SHA-256。
    $actual = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) {
        Stop-WithError "checksum verification failed for $archiveName (expected $expected, got $actual)"
    }
    Write-Info "checksum verified: $archiveName"

    # 4. 解压到隔离目录并确认其中的 depscope.exe 存在。
    $extractDir = Join-Path $workRoot 'extract'
    New-Item -ItemType Directory -Path $extractDir -Force | Out-Null
    try {
        Add-Type -AssemblyName System.IO.Compression.FileSystem -ErrorAction Stop
        [System.IO.Compression.ZipFile]::ExtractToDirectory($archivePath, $extractDir)
    }
    catch {
        Stop-WithError "could not extract $archiveName : $($_.Exception.Message)"
    }

    $staged = Get-ChildItem -LiteralPath $extractDir -Recurse -File -Filter "$BinaryName.exe" |
        Select-Object -First 1
    if (-not $staged) {
        Stop-WithError "archive $archiveName does not contain $BinaryName.exe"
    }

    # 5. 安装到用户目录；同目录暂存后替换，避免留下半写入的文件。
    if (-not (Test-Path -LiteralPath $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }
    $target = Join-Path $InstallDir "$BinaryName.exe"
    $stagingTarget = Join-Path $InstallDir (".$BinaryName.exe.new." + $PID)
    $replaced = Test-Path -LiteralPath $target

    try {
        Copy-Item -LiteralPath $staged.FullName -Destination $stagingTarget -Force
        Move-Item -LiteralPath $stagingTarget -Destination $target -Force
    }
    catch {
        if (Test-Path -LiteralPath $stagingTarget) {
            Remove-Item -LiteralPath $stagingTarget -Force -ErrorAction SilentlyContinue
        }
        Stop-WithError "could not install $BinaryName.exe to $target : $($_.Exception.Message)"
    }

    Write-Host ''
    Write-Host "DepScope $Version installed"
    Write-Host "  binary:  $target"
    if ($replaced) {
        Write-Host '  note:    replaced the previous depscope.exe at this path'
    }
    Write-Host ''

    if (Test-OnPath -Directory $InstallDir) {
        Write-Host 'Run "depscope version" to confirm the installation.'
    }
    else {
        Write-Host "$InstallDir is not on your PATH. Add it for the current user:"
        Write-Host ''
        Write-Host "  [Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path','User') + ';$InstallDir', 'User')"
        Write-Host ''
        Write-Host 'Then open a new terminal and run "depscope version".'
    }
}
finally {
    if (Test-Path -LiteralPath $workRoot) {
        Remove-Item -LiteralPath $workRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}
