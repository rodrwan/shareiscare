#!/bin/bash

# Script to run ShareIsCare unit tests

set -e  # Exit if there are errors

echo "=== Running ShareIsCare unit tests ==="

# Cleanup of temporary files that may exist from previous runs
echo "Cleaning temporary files..."
if [ -f config.yaml.bak ]; then
    mv config.yaml.bak config.yaml
fi

# Run the tests
echo "Running tests..."
go test -v ./...

# Verify that the code compiles for ARM (Raspberry Pi)
echo "Verifying compilation for Raspberry Pi..."
if GOOS=linux GOARCH=arm GOARM=7 go build -o /tmp/shareiscare-arm-test main.go; then
    echo "✓ Compilation for Raspberry Pi is correct"
    rm /tmp/shareiscare-arm-test
else
    echo "✗ Error in compilation for Raspberry Pi"
    exit 1
fi

echo "=== Tests completed ==="