#!/bin/bash

set -e

echo "════════════════════════════════════════════════════════"
echo "  L1 NODE FAILURE TEST RUNNER"
echo "════════════════════════════════════════════════════════"
echo ""

# Default values
L1_NODES=4
L2_NODES=1
ITERATIONS=50
L2_PORT=7000
NODE_TO_KILL="l1-node1"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -l1) L1_NODES="$2"; shift 2 ;;
        -l2) L2_NODES="$2"; shift 2 ;;
        -n) ITERATIONS="$2"; shift 2 ;;
        -port) L2_PORT="$2"; shift 2 ;;
        -kill) NODE_TO_KILL="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

echo "Configuration:"
echo "  L1 Nodes:      $L1_NODES"
echo "  L2 Nodes:      $L2_NODES"
echo "  Iterations:    $ITERATIONS"
echo "  L2 Port:       $L2_PORT"
echo "  Node to Kill:  $NODE_TO_KILL"
echo ""

# Build if needed
if [ ! -f "./bin/benchmark" ]; then
    echo "🔨 Building benchmark..."
    go build -o ./bin/benchmark .
    echo ""
fi

# Check if L1 and L2 are running
echo "🔍 Checking system status..."
if ! docker ps | grep -q "l1-node"; then
    echo "❌ Error: L1 nodes not running!"
    echo "   Please start L1 first: cd ../../layer-1 && make run NODES=$L1_NODES"
    exit 1
fi

if ! docker ps | grep -q "l2-shard"; then
    echo "❌ Error: L2 shards not running!"
    echo "   Please start L2 first: cd ../../layer-2 && make run NODES=$L2_NODES"
    exit 1
fi

echo "✅ System is running"
echo ""

# Create records directory
mkdir -p records

# Run the test
echo "🚀 Starting node failure test..."
echo ""

./bin/benchmark \
    -l1=$L1_NODES \
    -l2=$L2_NODES \
    -n=$ITERATIONS \
    -port=$L2_PORT \
    -kill=$NODE_TO_KILL

echo ""
echo "════════════════════════════════════════════════════════"
echo "  Test Complete!"
echo "════════════════════════════════════════════════════════"
echo ""
echo "📁 Results saved in ./records/"
echo ""
echo "💡 Next steps:"
echo "   1. Restart the killed node: docker start $NODE_TO_KILL"
echo "   2. Analyze results in ./records/"
echo ""