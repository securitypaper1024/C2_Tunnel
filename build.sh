#!/bin/bash

echo "========================================"
echo "  SecureTunnel Build Script"
echo "========================================"
echo

# 创建输出目录
mkdir -p build

echo "[1/4] Building for Windows AMD64..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o build/tunnel_windows_amd64.exe ./cmd/tunnel

echo "[2/4] Building for Linux AMD64..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o build/tunnel_linux_amd64 ./cmd/tunnel

echo "[3/4] Building for Linux ARM64..."
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o build/tunnel_linux_arm64 ./cmd/tunnel

echo "[4/4] Building for macOS AMD64..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o build/tunnel_darwin_amd64 ./cmd/tunnel

echo
echo "========================================"
echo "  Build Complete!"
echo "========================================"
echo
echo "Output files:"
ls -la build/

