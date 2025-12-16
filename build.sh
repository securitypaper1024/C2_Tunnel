#!/bin/bash

echo "========================================"
echo "  SecureTunnel Build Script"
echo "========================================"
echo

# 创建输出目录
mkdir -p build

echo "========================================"
echo "  Building Server"
echo "========================================"

echo "[1/4] Building Server for Windows AMD64..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o build/tunnel-server_windows_amd64.exe ./cmd/server

echo "[2/4] Building Server for Linux AMD64..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o build/tunnel-server_linux_amd64 ./cmd/server

echo "[3/4] Building Server for Linux ARM64..."
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o build/tunnel-server_linux_arm64 ./cmd/server

echo "[4/4] Building Server for macOS AMD64..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o build/tunnel-server_darwin_amd64 ./cmd/server

echo
echo "========================================"
echo "  Building Client"
echo "========================================"

echo "[1/4] Building Client for Windows AMD64..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o build/tunnel-client_windows_amd64.exe ./cmd/client

echo "[2/4] Building Client for Linux AMD64..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o build/tunnel-client_linux_amd64 ./cmd/client

echo "[3/4] Building Client for Linux ARM64..."
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o build/tunnel-client_linux_arm64 ./cmd/client

echo "[4/4] Building Client for macOS AMD64..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o build/tunnel-client_darwin_amd64 ./cmd/client

echo
echo "========================================"
echo "  Build Complete!"
echo "========================================"
echo
echo "Output files:"
ls -la build/
