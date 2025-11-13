@echo off
setlocal enabledelayedexpansion

echo ================================
echo Docker Environment Check
echo ================================
echo.

REM Check if Docker is installed
echo Checking if Docker is installed...
docker --version >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Docker not found
    echo.
    echo Docker is not installed. Please install Docker Desktop:
    echo   https://www.docker.com/products/docker-desktop/
    echo.
    pause
    exit /b 1
)
echo [OK] Docker found
echo.

REM Check if Docker daemon is running
echo Checking if Docker daemon is running...
docker ps >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Docker daemon is not running
    echo.
    echo Docker is installed but not running. Please:
    echo   1. Start Docker Desktop
    echo   2. Wait for Docker to fully start
    echo   3. Run this script again
    echo.
    pause
    exit /b 1
)
echo [OK] Docker daemon is running
echo.

REM Check Docker version
echo Docker version:
docker version --format "{{.Server.Version}}" 2>nul
echo.

REM Check if Redis image is available
echo Checking if Redis image is available...
docker images redis:7-alpine --format "{{.Repository}}" 2>nul | findstr /C:"redis" >nul 2>&1
if %errorlevel% equ 0 (
    echo [OK] Redis image already downloaded
) else (
    echo [WARN] Redis image not found
    echo Pulling Redis image...
    docker pull redis:7-alpine
    if %errorlevel% neq 0 (
        echo [ERROR] Failed to pull Redis image
        echo Please check your internet connection
        pause
        exit /b 1
    )
    echo [OK] Redis image downloaded
)
echo.

REM Test creating a container
echo Testing Docker container creation...
set TEST_CONTAINER=redis-test-%RANDOM%
docker run -d --name %TEST_CONTAINER% --rm redis:7-alpine >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Failed to create container
    pause
    exit /b 1
)
echo [OK] Container creation successful
docker stop %TEST_CONTAINER% >nul 2>&1
echo.

echo ================================
echo All checks passed!
echo ================================
echo.
echo You can now run the tests:
echo   go test -v ./...
echo   make test
echo.
pause
exit /b 0
