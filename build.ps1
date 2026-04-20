$GoTmp = "D:\tmp\gotmp"
if (-not (Test-Path $GoTmp)) { New-Item -ItemType Directory -Path $GoTmp -Force | Out-Null }
$env:GOTMPDIR = $GoTmp

$Version = git describe --tags --always --dirty 2>$null
if (-not $Version) { $Version = "dev" }

go build -ldflags "-X github.tools.sap/developer-relations/sap-devs-cli/cmd.Version=$Version" -o sap-devs.exe .
if ($LASTEXITCODE -eq 0) {
    Write-Host "Build OK: sap-devs.exe ($Version)" -ForegroundColor Green
} else {
    Write-Host "Build FAILED" -ForegroundColor Red
    exit 1
}

if (Test-Path "cmd\sap-devs-tray\go.mod") {
    # Copy cities.json for embedding in tray binary
    $citiesSrc = "internal\geo\cities.json"
    $citiesDst = "cmd\sap-devs-tray\data\cities.json"
    if (Test-Path $citiesSrc) {
        New-Item -ItemType Directory -Path (Split-Path $citiesDst) -Force | Out-Null
        Copy-Item $citiesSrc $citiesDst -Force
    }
    $env:CGO_ENABLED = "1"
    Push-Location cmd\sap-devs-tray
    go build -ldflags "-X main.version=$Version" -o ..\..\sap-devs-tray.exe .
    $rc = $LASTEXITCODE
    Pop-Location
    if ($rc -eq 0) {
        Write-Host "Build OK: sap-devs-tray.exe ($Version)" -ForegroundColor Green
    } else {
        Write-Host "Build FAILED: sap-devs-tray.exe (CGO required — need gcc in PATH)" -ForegroundColor Red
        exit 1
    }
}
