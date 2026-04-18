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
