#!/bin/bash

# Exit on error
set -e

# Output directory
mkdir -p dist

echo "Building for Linux/amd64..."
GOOS=linux GOARCH=amd64 go build -o dist/mcp-traceroute-linux-amd64 .

echo "Building for Linux/arm64..."
GOOS=linux GOARCH=arm64 go build -o dist/mcp-traceroute-linux-arm64 .

echo "Build complete. Artifacts in dist/"
ls -lh dist/
