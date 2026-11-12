@echo off
echo Building OmniFloat for Windows (native Glassmorphism launcher)...
go build -tags desktop,production -ldflags "-H windowsgui -s -w" -o build\bin\omnifloat.exe .
if %errorlevel% neq 0 (
    echo [ERROR] Build failed.
    exit /b %errorlevel%
)
echo [SUCCESS] Binary created at build\bin\omnifloat.exe
