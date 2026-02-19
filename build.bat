@echo off
echo Building rediscli...
go build -ldflags="-s -w" -o rediscli.exe .
if %ERRORLEVEL% EQU 0 (
    echo Build complete! Binary: rediscli.exe
    echo.
    echo Run with: rediscli.exe
) else (
    echo Build failed!
    exit /b 1
)
