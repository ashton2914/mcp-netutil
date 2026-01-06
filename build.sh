#!/bin/bash

# Exit on error
set -e

# Output directory
mkdir -p dist

echo "=== Building for Linux ==="
echo "Building for Linux/amd64..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/mcp-netutil-linux-amd64 .
echo "Building for Linux/arm64..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o dist/mcp-netutil-linux-arm64 .

echo -e "\n=== Building for Windows ==="
echo "Building for Windows/amd64..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/mcp-netutil-windows-amd64.exe .
echo "Building for Windows/arm64..."
CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -o dist/mcp-netutil-windows-arm64.exe .

echo -e "\n=== Building for macOS ==="
echo "Building for macOS/amd64 (Intel)..."
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o dist/mcp-netutil-darwin-amd64 .
echo "Building for macOS/arm64 (Apple Silicon)..."
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o dist/mcp-netutil-darwin-arm64 .

echo "Build complete. Artifacts in dist/"
ls -lh dist/
