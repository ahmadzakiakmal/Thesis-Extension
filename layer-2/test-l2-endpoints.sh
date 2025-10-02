#!/bin/bash

echo "🧪 Testing L2 Shards..."
echo ""

# Test Shard A
echo "📊 Shard A Health Check (Port 7000):"
echo -n "  GET /info: "
status=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:7000/info")
if [ "$status" = "200" ]; then
    echo "✅ ($status)"
    curl -s "http://localhost:7000/info" | jq -r '"  Shard: \(.shard_id) | Group: \(.client_group)"'
else
    echo "❌ ($status)"
fi
echo ""

# Test Shard B
echo "📊 Shard B Health Check (Port 7001):"
echo -n "  GET /info: "
status=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:7001/info")
if [ "$status" = "200" ]; then
    echo "✅ ($status)"
    curl -s "http://localhost:7001/info" | jq -r '"  Shard: \(.shard_id) | Group: \(.client_group)"'
else
    echo "❌ ($status)"
fi
echo ""

# Test L1 connectivity from L2 perspective
echo "🔗 L1 Connectivity Check:"
echo -n "  L1 Status: "
status=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:5000/l1/status")
if [ "$status" = "200" ]; then
    echo "✅ L1 is reachable"
else
    echo "❌ L1 is not reachable ($status)"
fi
echo ""

echo "🎉 Health Check Complete!"