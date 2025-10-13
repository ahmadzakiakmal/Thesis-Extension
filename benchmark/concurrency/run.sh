#!/bin/bash

echo "========================================"
echo "   CONCURRENCY BENCHMARK RUNNER"
echo "========================================"
echo ""

# Default values
WORKERS=100
DURATION=30
L1_NODES=4
L2_NODES=1
L2_PORT=7000
PACKAGE_ID="PKG-001"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -workers) WORKERS="$2"; shift 2 ;;
        -duration) DURATION="$2"; shift 2 ;;
        -l1) L1_NODES="$2"; shift 2 ;;
        -l2) L2_NODES="$2"; shift 2 ;; 
        -port) L2_PORT="$2"; shift 2 ;;
        -pkg) PACKAGE_ID="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

echo "Configuration:"
echo "  Workers:    $WORKERS"
echo "  Duration:   ${DURATION}s"
echo "  L1 Nodes:   $L1_NODES"
echo "  L2 Nodes:   $L2_NODES" 
echo "  L2 Port:    $L2_PORT"
echo "  Package ID: $PACKAGE_ID"
echo ""

# Build if binary doesn't exist
if [ ! -f "./bin/benchmark" ]; then
    echo "Building..."
    mkdir -p bin
    go build -o ./bin/benchmark .
    if [ $? -ne 0 ]; then
        echo "Build failed!"
        exit 1
    fi
    echo "✓ Build complete"
    echo ""
fi

# Create records directory
mkdir -p records

# Run benchmark
./bin/benchmark -workers=$WORKERS -duration=$DURATION -l1=$L1_NODES -l2=$L2_NODES -port=$L2_PORT -pkg=$PACKAGE_ID

echo ""
echo "Latest result:"
ls -lht records/ | head -n 2