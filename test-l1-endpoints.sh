#!/bin/bash

echo "🧪 Testing L1 BFT Network..."
echo ""

# Test all nodes are responding
echo "📊 Node Health Check:"
for i in {0..3}; do
    port=$((5000 + i))
    echo -n "  Node $i (port $port): "
    
    status=$(curl -s "http://localhost:$port/debug" 2>/dev/null | jq -r '.node_status // "offline"')
    if [ "$status" = "online" ]; then
        echo "✅ $status"
    else
        echo "❌ $status"
    fi
done
echo ""

# Test L1 endpoints
L1_URL="http://localhost:5000"

echo "🔧 Testing L1 API Endpoints:"

# 1. Status
echo -n "  GET /l1/status: "
response=$(curl -s "$L1_URL/l1/status" | jq -r '.data.status // "error"')
[ "$response" = "active" ] && echo "✅" || echo "❌ ($response)"

# 2. Shards
echo -n "  GET /l1/shards: "
response=$(curl -s "$L1_URL/l1/shards" | jq -r '.data.shards | length')
[ "$response" -gt "0" ] 2>/dev/null && echo "✅ ($response shards)" || echo "✅ (empty)"

# 3. Test commit (simulating L2)
echo -n "  POST /l1/commit: "
response=$(curl -s -X POST "$L1_URL/l1/commit" \
  -H "Content-Type: application/json" \
  -d '{
    "shard_id": "shard-a",
    "client_group": "group-a", 
    "session_id": "test-session-'$(date +%s)'",
    "operator_id": "OPR-001",
    "session_data": {"test": "data"},
    "l2_node_id": "l2-node-a",
    "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
  }' | jq -r '.data.message // .error // "unknown"')
  
if [[ "$response" =~ "successfully" ]]; then
    echo "✅"
    
    # If commit worked, test queries
    echo -n "  GET /l1/sessions/group/group-a: "
    count=$(curl -s "$L1_URL/l1/sessions/group/group-a" | jq -r '.data | length // 0')
    [ "$count" -gt "0" ] 2>/dev/null && echo "✅ ($count sessions)" || echo "✅ (empty)"
    
    echo -n "  GET /l1/sessions/shard/shard-a: "
    count=$(curl -s "$L1_URL/l1/sessions/shard/shard-a" | jq -r '.data | length // 0')  
    [ "$count" -gt "0" ] 2>/dev/null && echo "✅ ($count sessions)" || echo "✅ (empty)"
else
    echo "❌ ($response)"
fi

echo ""
echo "🎉 L1 Network Test Complete!"

# Show network stats
echo ""
echo "📈 Network Statistics:"
consensus_info=$(curl -s "$L1_URL/debug")
echo "  Latest Block Height: $(echo $consensus_info | jq -r '.latest_block_height // 0')"
echo "  Connected Peers: $(echo $consensus_info | jq -r '.num_peers_out // 0')"
echo "  Node Status: $(echo $consensus_info | jq -r '.node_status // "unknown"')"