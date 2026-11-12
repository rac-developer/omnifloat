Write-Host "Building OmniFloat (native Glassmorphism launcher)..." -ForegroundColor Cyan
go build -tags desktop,production -ldflags "-H windowsgui -s -w" -o build/bin/omnifloat.exe .
if ($LASTEXITCODE -eq 0) {
    Write-Host "[SUCCESS] Binary compiled at build/bin/omnifloat.exe" -ForegroundColor Green
} else {
    Write-Host "[ERROR] Build failed." -ForegroundColor Red
}
