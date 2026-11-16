Write-Host "Building OmniFloat (native Glassmorphism launcher)..." -ForegroundColor Cyan
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -tags desktop,production -ldflags "-H windowsgui -s -w" -o build/bin/omnifloat.exe .
if ($LASTEXITCODE -eq 0) {
    Write-Host "[SUCCESS] Binary compiled at build/bin/omnifloat.exe" -ForegroundColor Green
} else {
    Write-Host "[ERROR] Build failed." -ForegroundColor Red
}
