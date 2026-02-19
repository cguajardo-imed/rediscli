#!/bin/bash
set -e

echo "Building rediscli..."
go build -ldflags="-s -w -X main.Version=$(git describe --tags --always)" -o rediscli .

echo "Build complete! Binary: rediscli"
echo ""
echo "Run with: ./rediscli"
