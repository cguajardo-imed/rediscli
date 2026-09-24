@echo off
echo Building gns-cli...
go build -ldflags="-s -w" -o gns-cli.exe .
if %ERRORLEVEL% EQU 0 (
    echo Build complete! Binary: gns-cli.exe
    echo.
    echo Run with: gns-cli.exe
) else (
    echo Build failed!
    exit /b 1
)
