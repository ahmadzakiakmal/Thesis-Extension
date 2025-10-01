#!/bin/bash

echo "📈 L1 BFT Network Monitor"
echo ""

# Check all nodes
echo "🔍 Node Status:"
for i in {0..3}; do
    port=$((5000 + i))
    
    info=$(curl -s "http://localhost:$port/debug" 2>/dev/null)
    status=$(echo $info | jq -r '.node_status // "offline"')
    peers=$(echo $info | jq -r '.num_peers_out // 0')
    height=$(echo $info | jq -r '.latest_block_height // 0')
    
    case $status in
        "online") status_icon="✅" ;;
        "syncing") status_icon="🔄" ;;
        *) status_icon="❌" ;;
    esac
    
    echo "  Node $i: $status_icon $status | Peers: $peers | Block: $height"
done

echo ""
echo "🌐 Network Info:"
network_info=$(curl -s "http://localhost:5000/debug" 2>/dev/null)
echo "  Latest Block: $(echo $network_info | jq -r '.latest_block_height // 0')"
echo "  Block Time: $(echo $network_info | jq -r '.latest_block_time // "unknown"')"
echo "  Catching Up: $(echo $network_info | jq -r '.catching_up // false')"

echo ""
echo "📊 Database Status:"
shards=$(curl -s "http://localhost:5000/l1/shards" 2>/dev/null | jq -r '.data.shards | length // 0')
echo "  Registered Shards: $shards"

for group in "group-a" "group-b"; do
    sessions=$(curl -s "http://localhost:5000/l1/sessions/group/$group" 2>/dev/null | jq -r '.data | length // 0')
    echo "  Sessions ($group): $sessions"
done

echo ""
echo "⏱️  Monitor completed at $(date)"