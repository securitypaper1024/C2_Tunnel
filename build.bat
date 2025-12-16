@echo off
echo ========================================
echo   SecureTunnel Build Script
echo ========================================
echo.

REM 创建输出目录
if not exist "build" mkdir build

echo ========================================
echo   Building Server
echo ========================================

echo [1/3] Building Server for Windows AMD64...
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-s -w" -o build\tunnel-server_windows_amd64.exe .\cmd\server

echo [2/3] Building Server for Linux AMD64...
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-s -w" -o build\tunnel-server_linux_amd64 .\cmd\server

echo [3/3] Building Server for macOS AMD64...
set GOOS=darwin
set GOARCH=amd64
go build -ldflags="-s -w" -o build\tunnel-server_darwin_amd64 .\cmd\server

echo.
echo ========================================
echo   Building Client
echo ========================================

echo [1/3] Building Client for Windows AMD64...
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-s -w" -o build\tunnel-client_windows_amd64.exe .\cmd\client

echo [2/3] Building Client for Linux AMD64...
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-s -w" -o build\tunnel-client_linux_amd64 .\cmd\client

echo [3/3] Building Client for macOS AMD64...
set GOOS=darwin
set GOARCH=amd64
go build -ldflags="-s -w" -o build\tunnel-client_darwin_amd64 .\cmd\client

echo.
echo ========================================
echo   Build Complete!
echo ========================================
echo.
echo Output files:
dir /b build\

pause
