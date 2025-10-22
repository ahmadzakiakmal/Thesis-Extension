#!/bin/bash

set -e

echo "════════════════════════════════════════════════════════"
echo "  L1 BYZANTINE FAULT TEST RUNNER"
echo "════════════════════════════════════════════════════════"
echo ""

# Default values
L1_NODES=19
L2_NODES=1
ITERATIONS=50
L2_PORT=7000
BYZANTINE_1="l1-node1"
BYZANTINE_2="l1-node2"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -l1) L1_NODES="$2"; shift 2 ;;
        -l2) L2_NODES="$2"; shift 2 ;;
        -n) ITERATIONS="$2"; shift 2 ;;
        -port) L2_PORT="$2"; shift 2 ;;
        -byz1) BYZANTINE_1="$2"; shift 2 ;;
        -byz2) BYZANTINE_2="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

echo "Configuration:"
echo "  L1 Nodes:         $L1_NODES"
echo "  L2 Nodes:         $L2_NODES"
echo "  Iterations:       $ITERATIONS"
echo "  L2 Port:          $L2_PORT"
echo "  Byzantine Node 1: $BYZANTINE_1"
echo "  Byzantine Node 2: $BYZANTINE_2"
echo ""

# Calculate fault tolerance
FAULT_TOLERANCE=$(( (L1_NODES - 1) / 3 ))
echo "  Fault Tolerance:  f=$FAULT_TOLERANCE (can tolerate $FAULT_TOLERANCE Byzantine nodes)"
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

# Verify Byzantine nodes exist
echo "🔍 Verifying Byzantine nodes..."
if ! docker ps | grep -q "$BYZANTINE_1"; then
    echo "❌ Error: Byzantine node 1 ($BYZANTINE_1) not found!"
    exit 1
fi

if ! docker ps | grep -q "$BYZANTINE_2"; then
    echo "❌ Error: Byzantine node 2 ($BYZANTINE_2) not found!"
    exit 1
fi

echo "✅ Byzantine nodes verified"
echo ""

# Create records directory
mkdir -p records

# Run the test
echo "🚀 Starting Byzantine fault test..."
echo ""

./bin/benchmark \
    -l1=$L1_NODES \
    -l2=$L2_NODES \
    -n=$ITERATIONS \
    -port=$L2_PORT \
    -byz1=$BYZANTINE_1 \
    -byz2=$BYZANTINE_2

echo ""
echo "════════════════════════════════════════════════════════"
echo "  Test Complete!"
echo "════════════════════════════════════════════════════════"
echo ""
echo "📁 Results saved in ./records/"
echo ""
echo "💡 Next steps:"
echo "   1. Restart Byzantine nodes: docker start $BYZANTINE_1 $BYZANTINE_2"
echo "   2. Analyze results in ./records/"
echo "   3. Run: python3 preview.py"
echo ""