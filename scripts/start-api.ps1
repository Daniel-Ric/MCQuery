$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$Config = Join-Path $Root "config\api.local.json"
if (-not (Test-Path -LiteralPath $Config)) {
    $Config = Join-Path $Root "config\api.example.json"
}
$Exe = Join-Path $Root "uwp-tcp-con.exe"
$BareExe = Join-Path $Root "uwp-tcp-con"
if (Test-Path -LiteralPath $Exe) {
    & $Exe --api --api-config $Config
} elseif (Test-Path -LiteralPath $BareExe) {
    & $BareExe --api --api-config $Config
} else {
    Push-Location $Root
    try {
        go run ./cmd/uwp-tcp-con --api --api-config $Config
    } finally {
        Pop-Location
    }
}
