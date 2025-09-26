#!/bin/bash

# Simple L1 network setup for development
NODE_COUNT=4
BASE_P2P_PORT=9000
BASE_RPC_PORT=9001
BASE_HTTP_PORT=5000
BASE_DIR="./node-config"
BASE_POSTGRES_PORT=5440

# Parse node count if provided
while getopts ":n:" opt; do
    case $opt in
    n) NODE_COUNT="$OPTARG" ;;
    \?)
        echo "Usage: $0 [-n NODE_COUNT]"
        exit 1
        ;;
    esac
done

echo "🚀 Setting up L1 BFT network with $NODE_COUNT nodes..."

# Validate BFT requirements
if [ $NODE_COUNT -lt 4 ]; then
    echo "⚠️  Warning: At least 4 nodes recommended for BFT (you have $NODE_COUNT)"
fi

# Clean and create directories
rm -rf "$BASE_DIR"
mkdir -p "$BASE_DIR"

# Initialize all nodes
for i in $(seq 0 $((NODE_COUNT - 1))); do
    mkdir -p "$BASE_DIR/node$i"
    cometbft init --home="$BASE_DIR/node$i"
    
    # Configure each node
    p2p_port=$((BASE_P2P_PORT + i * 2))
    rpc_port=$((BASE_RPC_PORT + i * 2))
    
    # Update config
    sed -i.bak "s/^moniker = \".*\"/moniker = \"l1-node$i\"/" "$BASE_DIR/node$i/config/config.toml"
    sed -i.bak "s/^laddr = \"tcp:\/\/0.0.0.0:26656\"/laddr = \"tcp:\/\/0.0.0.0:$p2p_port\"/" "$BASE_DIR/node$i/config/config.toml"
    sed -i.bak "s/^laddr = \"tcp:\/\/127.0.0.1:26657\"/laddr = \"tcp:\/\/0.0.0.0:$rpc_port\"/" "$BASE_DIR/node$i/config/config.toml"
    sed -i.bak 's/^create_empty_blocks = true/create_empty_blocks = false/' "$BASE_DIR/node$i/config/config.toml"
    sed -i.bak 's/^addr_book_strict = true/addr_book_strict = false/' "$BASE_DIR/node$i/config/config.toml"
    sed -i.bak 's/^cors_allowed_origins = \[\]/cors_allowed_origins = ["*"]/' "$BASE_DIR/node$i/config/config.toml"
    
    echo "✅ Node $i configured (P2P: $p2p_port, RPC: $rpc_port)"
done

# Create shared genesis with all validators
echo "🔗 Creating shared genesis file..."
cp "$BASE_DIR/node0/config/genesis.json" "$BASE_DIR/updated_genesis.json"

for i in $(seq 1 $((NODE_COUNT - 1))); do
    NODE_PUBKEY=$(cat "$BASE_DIR/node$i/config/priv_validator_key.json" | jq -r '.pub_key.value')
    cat "$BASE_DIR/updated_genesis.json" | jq --arg pubkey "$NODE_PUBKEY" --arg name "l1-node$i" \
        '.validators += [{"address":"","pub_key":{"type":"tendermint/PubKeyEd25519","value":$pubkey},"power":"10","name":$name}]' >"$BASE_DIR/temp_genesis.json"
    mv "$BASE_DIR/temp_genesis.json" "$BASE_DIR/updated_genesis.json"
done

# Share genesis to all nodes
for i in $(seq 0 $((NODE_COUNT - 1))); do
    cp "$BASE_DIR/updated_genesis.json" "$BASE_DIR/node$i/config/genesis.json"
done

# Configure peer connections
echo "🌐 Configuring peer connections..."
declare -a NODE_IDS
for i in $(seq 0 $((NODE_COUNT - 1))); do
    NODE_IDS[$i]=$(cometbft show-node-id --home="$BASE_DIR/node$i")
done

for i in $(seq 0 $((NODE_COUNT - 1))); do
    PEERS=""
    for j in $(seq 0 $((NODE_COUNT - 1))); do
        if [ $i -ne $j ]; then
            p2p_port=$((BASE_P2P_PORT + j * 2))
            if [ -z "$PEERS" ]; then
                PEERS="${NODE_IDS[$j]}@l1-node${j}:$p2p_port"
            else
                PEERS="$PEERS,${NODE_IDS[$j]}@l1-node${j}:$p2p_port"
            fi
        fi
    done
    sed -i.bak "s/^persistent_peers = \"\"/persistent_peers = \"$PEERS\"/" "$BASE_DIR/node$i/config/config.toml"
done

# Generate simple docker-compose.yml
echo "🐳 Generating docker-compose.yml..."
cat > "./docker-compose.yml" << EOL
services:
EOL

for i in $(seq 0 $((NODE_COUNT - 1))); do
    p2p_port=$((BASE_P2P_PORT + i * 2))
    rpc_port=$((BASE_RPC_PORT + i * 2))
    http_port=$((BASE_HTTP_PORT + i))
    postgres_port=$((BASE_POSTGRES_PORT + i))

    cat >> "./docker-compose.yml" << EOL
  l1-node$i:
    image: l1-node:latest
    container_name: l1-node$i
    ports:
      - "$http_port:$http_port"
      - "$p2p_port:$p2p_port"
      - "$rpc_port:$rpc_port"
    volumes:
      - $BASE_DIR/node$i:/root/.cometbft
    command: 
      - "/app/bin"
      - "--cmt-home=/root/.cometbft"
      - "--http-port=$http_port"
      - "--postgres-host=l1-postgres$i:5432"
    depends_on:
      - l1-postgres$i
    networks:
      - l1-network

  l1-postgres$i:
    image: postgres:14
    container_name: l1-postgres$i
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: l1db
    volumes:
      - l1-postgres-data$i:/var/lib/postgresql/data
    ports:
      - "$postgres_port:5432"
    networks:
      - l1-network

EOL
done

cat >> "./docker-compose.yml" << EOL
networks:
  l1-network:
    driver: bridge

volumes:
EOL

for i in $(seq 0 $((NODE_COUNT - 1))); do
  cat >> "./docker-compose.yml" << EOL
  l1-postgres-data$i:
EOL
done

# Fix permissions
sudo chown -R $(id -u):$(id -g) node-config/ 2>/dev/null || true
sudo chmod -R 755 node-config/ 2>/dev/null || true

# Clear data directories
for i in $(seq 0 $((NODE_COUNT - 1))); do
    mkdir -p "$BASE_DIR/node$i/data"
    rm -rf "$BASE_DIR/node$i/data/"*
    echo '{"height": "0", "round": 0, "step": 0}' > "$BASE_DIR/node$i/data/priv_validator_state.json"
done

echo ""
echo "🎉 L1 BFT Network setup complete!"
echo ""
echo "📊 Network Info:"
echo "   Nodes: $NODE_COUNT"
echo "   Fault Tolerance: $((NODE_COUNT / 3)) Byzantine faults"
echo ""
echo "🔗 API Endpoints:"
for i in $(seq 0 $((NODE_COUNT - 1))); do
    http_port=$((BASE_HTTP_PORT + i))
    echo "   Node $i: http://localhost:$http_port"
done
echo ""
echo "▶️  Run: make run"
echo "🧪 Test: make test"
echo "📈 Monitor: make monitor"