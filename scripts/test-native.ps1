# Native PVF tests with a private, checksum-pinned local profile.
[CmdletBinding()]
param(
    [string]$Profile = '',
    [string]$Archive = '',
    [string]$GoExecutable = '',
    [string]$Run = '',
    [string[]]$Packages = @('./...'),
    [int]$ParallelPackages = 1,
    [switch]$Full,
    [switch]$PrepareFixtures,
    [switch]$ArchiveMutationProof,
    [switch]$ExtendedIntegration,
    [switch]$ClientRoundTrip,
    [string]$JsonLog = ''
)
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$moduleRoot = Join-Path $repoRoot 'server/work/dfo-lan'
if (-not $Profile) { $Profile = Join-Path $moduleRoot 'runtime/test/native.local.json' }
$config = @{}
if (Test-Path -LiteralPath $Profile) {
    $config = Get-Content -LiteralPath $Profile -Raw | ConvertFrom-Json -AsHashtable
}
if (-not $Archive) { $Archive = $config.Archive }
if (-not $GoExecutable) { $GoExecutable = $config.GoExecutable }
if (-not $GoExecutable) { $GoExecutable = 'go' }
if (-not $Archive) { throw 'Specify -Archive for the current inner Script PVF, or create the local profile.' }
if ($ParallelPackages -lt 1) { throw 'ParallelPackages must be positive.' }
$Archive = (Resolve-Path -LiteralPath $Archive).Path
$checksum = (Get-FileHash -LiteralPath $Archive -Algorithm SHA256).Hash.ToLowerInvariant()
if ($config.ArchiveSHA256 -and $config.ArchiveSHA256 -ne $checksum) {
    throw "Archive checksum changed; inspect the new archive and update the private profile: $checksum"
}
if ($PrepareFixtures) {
    & (Join-Path $PSScriptRoot 'prepare-native-test-fixtures.ps1') -Archive $Archive -GoExecutable $GoExecutable
}
$vars = @{
    GOTOOLCHAIN = 'local'
    DFO_PVF_CORE_TEST_ARCHIVE = $Archive
    DFO_PVF_CORE_TEST_SHA256 = $checksum
    DFO_PVF_SCOPE_TEST_ARCHIVE = $Archive
    DFO_PVF_SCOPE_TEST_SHA256 = $checksum
    DFO_LOOT_PVF = $Archive
    DFO_REPAIR_PVF = $Archive
    DFO_BORDER_PVF = $Archive
    DFO_ORDINARY_PVF_PREPARE = $Archive
    DFO_ELITE_MOON_TEST_ARCHIVE = $Archive
    DFO_ELITE_MOON_TEST_SHA256 = $checksum
    DFO_ELITE_ODYSSEY_TEST_ARCHIVE = $Archive
    DFO_ELITE_ODYSSEY_TEST_SHA256 = $checksum
    DFO_ELITE_STORY_TEST_ARCHIVE = $Archive
    DFO_ELITE_STORY_TEST_SHA256 = $checksum
    US115_TEST_BOOST_PVF = $Archive
    US115_TEST_BOOST_PVF_SHA256 = $checksum
    DFO_PVF_CACHE_DIR = (Join-Path $moduleRoot 'runtime/pvf-cache')
    DFO_PVF_CACHE_IDENTITY = 'dfo-lan-test-cache-v1'
    DFO_PVF_ARCHIVE_FULL_SWEEP = $(if ($Full) { '1' } else { '0' })
    # Host/WFP tests belong to a separate, explicit Windows integration run.
    DFO_CLIENT_HOST_LIVE_TEST = '0'
    DFO_WFP_LIVE_TEST = '0'
    ATTUNEMENT_REWARD_INTEGRATION = $(if ($ExtendedIntegration) { '1' } else { '0' })
    BORDER_DROP_INTEGRATION = $(if ($ExtendedIntegration) { '1' } else { '0' })
    DFO_EXTENDED_CATALOG_INTEGRATION = $(if ($ExtendedIntegration) { '1' } else { '0' })
    EQUIPMENT_INTEGRATION = $(if ($ExtendedIntegration) { '1' } else { '0' })
    CONFIG_SWEEP = $(if ($ExtendedIntegration) { '1' } else { '0' })
}
if ($config.OldArchive) {
    $oldArchive = (Resolve-Path -LiteralPath $config.OldArchive).Path
    $oldChecksum = (Get-FileHash -LiteralPath $oldArchive -Algorithm SHA256).Hash.ToLowerInvariant()
    if (-not $config.OldArchiveSHA256 -or $oldChecksum -ne $config.OldArchiveSHA256) { throw 'Old archive checksum is missing or changed.' }
    if ($oldChecksum -eq $checksum) { throw 'Compatibility audit requires two different archive versions.' }
    $vars.DFO_COMPAT_OLD_ARCHIVE = $oldArchive
    $vars.DFO_COMPAT_NEW_ARCHIVE = $Archive
}
if ($ArchiveMutationProof) {
    $proofRoot = Join-Path $moduleRoot 'runtime/test/archive-proof'
    [IO.Directory]::CreateDirectory($proofRoot) | Out-Null
    $vars.PVF_ADDENTRY_SOURCE = $Archive
    $vars.PVF_ADDENTRY_OUT = Join-Path $proofRoot 'Script.addentry-test.pvf'
}
if ($ClientRoundTrip) {
    if (-not $config.WrapClientDirectory -or -not $config.WrapInner) { throw 'Configure a private client snapshot and its unwrapped inner archive before ClientRoundTrip.' }
    $snapshotFiles = @{
        'DFO.exe' = $config.WrapClientExeSHA256
        'sk.dat' = $config.WrapClientKeysSHA256
        'Script.pvf' = $config.WrapOuterSHA256
    }
    foreach ($name in $snapshotFiles.Keys) {
        $actual = (Get-FileHash -LiteralPath (Join-Path $config.WrapClientDirectory $name) -Algorithm SHA256).Hash.ToLowerInvariant()
        if (-not $snapshotFiles[$name] -or $snapshotFiles[$name] -ne $actual) { throw "Client snapshot checksum missing or changed: $name" }
    }
    $inner = (Resolve-Path -LiteralPath $config.WrapInner).Path
    $actual = (Get-FileHash -LiteralPath $inner -Algorithm SHA256).Hash.ToLowerInvariant()
    if (-not $config.WrapInnerSHA256 -or $config.WrapInnerSHA256 -ne $actual) { throw 'Client snapshot inner archive checksum missing or changed.' }
    $vars.PVF_WRAP_CLIENT = $config.WrapClientDirectory
    $vars.PVF_WRAP_INNER = $inner
    $vars.PVF_WRAP_OUT = Join-Path $moduleRoot 'runtime/test/Script.client-roundtrip-test.pvf'
    if ([string]::Equals($inner, $vars.PVF_WRAP_OUT, [StringComparison]::OrdinalIgnoreCase)) { throw 'Roundtrip output must differ from its input.' }
}
$fixtureRoot = Join-Path $moduleRoot 'runtime/test/source-exports'
$readyPath = Join-Path $fixtureRoot 'ready.json'
if (Test-Path -LiteralPath $readyPath) {
    $ready = Get-Content -LiteralPath $readyPath -Raw | ConvertFrom-Json
    if ($ready.ArchiveSHA256 -ne $checksum) { throw 'Exported test fixtures belong to another archive; rerun with -PrepareFixtures.' }
    $vars.US115_TEST_BOOST_SOURCE = $fixtureRoot
    $vars.US115_TEST_BOOST_CHALLENGE = Join-Path $fixtureRoot 'challenge'
    $vars.US115_TEST_BOOST_BUFF_SOURCE = Join-Path $fixtureRoot 'buff'
    $vars.US115_TEST_BOOST_SPECIAL_SOURCE = Join-Path $fixtureRoot 'special'
}
$previous = @{}
foreach ($key in $vars.Keys) {
    $previous[$key] = [Environment]::GetEnvironmentVariable($key, 'Process')
    [Environment]::SetEnvironmentVariable($key, $vars[$key], 'Process')
}
$goArgs = @('test', '-count=1', '-p', "$ParallelPackages", '-timeout', '45m')
if ($Run) { $goArgs += @('-run', $Run) }
if ($JsonLog) {
    $JsonLog = [IO.Path]::GetFullPath($JsonLog)
    [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($JsonLog)) | Out-Null
    $goArgs += '-json'
}
$goArgs += $Packages
Write-Host "Native archive SHA256: $checksum"
Write-Host "Go: $GoExecutable $($goArgs -join ' ')"
Push-Location $moduleRoot
try {
    if ($JsonLog) { & $GoExecutable @goArgs | Out-File -LiteralPath $JsonLog -Encoding utf8 }
    else { & $GoExecutable @goArgs }
    $result = $LASTEXITCODE
}
finally {
    Pop-Location
    foreach ($key in $vars.Keys) { [Environment]::SetEnvironmentVariable($key, $previous[$key], 'Process') }
}
exit $result
