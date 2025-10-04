#!/bin/bash

# L2 Shard Network Setup Script
# Generates docker-compose.yml for configurable number of L2 shards

set -e

# Default values
NODE_COUNT=2
BASE_HTTP_PORT=7000
BASE_POSTGRES_PORT=5433

# Parse command line arguments
while getopts "n:" opt; do
  case $opt in
    n) NODE_COUNT=$OPTARG ;;
    *) echo "Usage: $0 -n <number_of_nodes>" >&2; exit 1 ;;
  esac
done

echo "=========================================="
echo "  L2 Shard Network Setup"
echo "=========================================="
echo "Number of L2 shards: $NODE_COUNT"
echo ""

# Generate shard names (a, b, c, d, e, f, ...)
SHARD_LETTERS=("a" "b" "c" "d" "e" "f" "g" "h" "i" "j" "k" "l" "m" "n" "o" "p")

# Start generating docker-compose.yml
cat > "./docker-compose.yml" << 'EOL'
services:
EOL

# Generate each L2 shard
for i in $(seq 0 $((NODE_COUNT - 1))); do
    SHARD_LETTER=${SHARD_LETTERS[$i]}
    SHARD_ID="shard-${SHARD_LETTER}"
    CLIENT_GROUP="group-${SHARD_LETTER}"
    L2_NODE_ID="l2-node-${SHARD_LETTER}"
    HTTP_PORT=$((BASE_HTTP_PORT + i))
    POSTGRES_PORT=$((BASE_POSTGRES_PORT + i))
    
    echo "Configuring: $SHARD_ID (Group: $CLIENT_GROUP) on port $HTTP_PORT"
    
    cat >> "./docker-compose.yml" << EOL
  l2-${SHARD_ID}:
    build: .
    container_name: l2-${SHARD_ID}
    environment:
      SHARD_ID: "${SHARD_ID}"
      CLIENT_GROUP: "${CLIENT_GROUP}"
      L2_NODE_ID: "${L2_NODE_ID}"
      HTTP_PORT: "7000"
      DB_HOST: "l2-postgres-${SHARD_LETTER}"
      DB_PORT: "5432"
      DB_USER: "postgres"
      DB_PASS: "postgrespassword"
      DB_NAME: "l2_shard_db"
      L1_ENDPOINT: "http://l1-node0:5000"
    ports:
      - "${HTTP_PORT}:7000"
    networks:
      - layer-1_l1-network
    depends_on:
      - l2-postgres-${SHARD_LETTER}

  l2-postgres-${SHARD_LETTER}:
    image: postgres:14
    container_name: l2-postgres-${SHARD_LETTER}
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgrespassword
      POSTGRES_DB: l2_shard_db
    volumes:
      - l2-postgres-data-${SHARD_LETTER}:/var/lib/postgresql/data
    ports:
      - "${POSTGRES_PORT}:5432"
    networks:
      - layer-1_l1-network

EOL
done

# Add networks section
cat >> "./docker-compose.yml" << EOL
networks:
  layer-1_l1-network:
    external: true

volumes:
EOL

# Add volumes
for i in $(seq 0 $((NODE_COUNT - 1))); do
    SHARD_LETTER=${SHARD_LETTERS[$i]}
    cat >> "./docker-compose.yml" << EOL
  l2-postgres-data-${SHARD_LETTER}:
EOL
done

echo ""
echo "✅ L2 Shard Network setup complete!"
echo "   Generated docker-compose.yml with $NODE_COUNT shards"
echo ""
echo "Shards configured:"
for i in $(seq 0 $((NODE_COUNT - 1))); do
    SHARD_LETTER=${SHARD_LETTERS[$i]}
    HTTP_PORT=$((BASE_HTTP_PORT + i))
    echo "  - shard-${SHARD_LETTER} (group-${SHARD_LETTER}): http://localhost:${HTTP_PORT}"
done
echo ""