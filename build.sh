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

echo "Build complete. Artifacts in dist/"
ls -lh dist/
