#!/bin/bash

echo "========================================"
echo "   CROSS-SHARD LATENCY BENCHMARK"
echo "========================================"
echo ""

ITERATIONS=100
L1_NODES=4
L2_NODES=1
L2_PORT=7000

while [[ $# -gt 0 ]]; do
    case $1 in
        -n) ITERATIONS="$2"; shift 2 ;;
        -l1) L1_NODES="$2"; shift 2 ;;
        -l2) L2_NODES="$2"; shift 2 ;; 
        -port) L2_PORT="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

echo "Configuration:"
echo "  Iterations: $ITERATIONS"
echo "  L1 Nodes:   $L1_NODES"
echo "  L2 Nodes:   $L2_NODES" 
echo "  L2 Port:    $L2_PORT (Shard A)"
echo "  Header:     X-Client-Group: group-b (CROSS-SHARD)"
echo ""

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

mkdir -p records

./bin/benchmark -n $ITERATIONS -l1 $L1_NODES -l2 $L2_NODES -port $L2_PORT

echo ""
echo "Latest result:"
ls -lht records/ | head -n 2