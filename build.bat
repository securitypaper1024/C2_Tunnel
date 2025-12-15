@echo off
echo ========================================
echo   SecureTunnel Build Script
echo ========================================
echo.

REM 创建输出目录
if not exist "build" mkdir build

echo [1/3] Building for Windows AMD64...
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-s -w" -o build\tunnel_windows_amd64.exe .\cmd\tunnel

echo [2/3] Building for Linux AMD64...
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-s -w" -o build\tunnel_linux_amd64 .\cmd\tunnel

echo [3/3] Building for macOS AMD64...
set GOOS=darwin
set GOARCH=amd64
go build -ldflags="-s -w" -o build\tunnel_darwin_amd64 .\cmd\tunnel

echo.
echo ========================================
echo   Build Complete!
echo ========================================
echo.
echo Output files:
dir /b build\

pause

