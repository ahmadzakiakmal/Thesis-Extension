# Layer 1 - BFT Consensus for Sharded L2

A simplified Byzantine Fault Tolerant consensus layer that handles commits from multiple L2 shards.

## Quick Start

```bash
# Full setup (clean + build + configure + start)
make run

# Quick start (auto-configures if needed, skips build) ⚡
make start

# Just build everything
make build

# Test all endpoints
make test

# Monitor network health
make monitor

# Stop everything
make clean
```

## What This Does

- **🏗️ Builds** Go binary and Docker image
- **🔗 Configures** 4-node BFT network with CometBFT
- **🗄️ Sets up** PostgreSQL databases for each node
- **🚀 Starts** everything in Docker containers
- **✅ Tests** all L1 API endpoints

## L1 API Endpoints

| Endpoint | Purpose |
|----------|---------|
| `POST /l1/commit` | Receive commits from L2 shards |
| `GET /l1/sessions/group/{group}` | Query sessions by client group |
| `GET /l1/sessions/shard/{shard}` | Query sessions by shard |
| `GET /l1/transaction/{hash}` | Get transaction details |
| `GET /l1/status` | Get L1 system status |
| `GET /l1/shards` | Get registered shards |
| `GET /debug` | Debug information |

## Network Access

After `make run` or `make start`, your L1 nodes are available at:
- Node 0: http://localhost:5000
- Node 1: http://localhost:5001  
- Node 2: http://localhost:5002
- Node 3: http://localhost:5003

## L2 Integration

Your L2 shards should commit completed sessions to L1:

```bash
curl -X POST http://localhost:5000/l1/commit \
  -H "Content-Type: application/json" \
  -d '{
    "shard_id": "shard-a",
    "client_group": "group-a",
    "session_id": "session-123",
    "operator_id": "OPR-001", 
    "session_data": {...},
    "l2_node_id": "l2-node-a",
    "timestamp": "2024-01-01T12:00:00Z"
  }'
```

## Architecture

```
L2 Shards → L1 BFT Consensus → Immutable Ledger
    ↓              ↓                    ↓
Individual    Byzantine Fault      Cross-shard
Sessions      Tolerant             Verification
              Validation           & Queries
```

## Makefile Commands

| Command | Purpose | When to Use |
|---------|---------|-------------|
| `make run` | Full setup: clean + build + start | First time or after code changes |
| `make start` | Quick start (auto-setup if needed) | Fast restarts, daily development ⚡ |
| `make build` | Build binary and Docker image only | When you only want to compile |
| `make test` | Test L1 endpoints | Verify the network works |
| `make monitor` | Check network health | Monitor running system |
| `make clean` | Stop and cleanup everything | Fresh start needed |
| `make debug` | Run single node for debugging | Local debugging without Docker |

## Development Workflow

**First Time Setup:**
```bash
make run              # Full setup (~30s)
```

**Daily Development (Fast!):**
```bash
# Stop containers
docker-compose down

# Quick restart (1-2s) ⚡
make start
```

**After Code Changes:**
```bash
make build            # Rebuild (~15s)
make start            # Restart (1-2s)

# Or do both at once
make run              # Clean + build + start (~30s)
```

**Change Number of Nodes:**
```bash
make run NODES=7      # 7-node BFT network
make run NODES=10     # 10-node BFT network
```

## Pro Tips

- 💡 **Use `make start` for daily work** - it's 10-15x faster than `make run`
- 💡 **`make start` is smart** - auto-generates config on first run
- 💡 **`docker-compose down` doesn't delete config** - safe to restart quickly
- 💡 **`make clean` removes everything** - use when you want a fresh slate
- 💡 **Config is cached** - only regenerated when needed

## Requirements

- Go 1.24+
- Docker & Docker Compose
- jq (for JSON processing in scripts)
- CometBFT (included in container)

## Troubleshooting

**"No such file or directory: config.toml"**
```bash
make start    # Will auto-generate missing config
```

**Containers won't start:**
```bash
make clean    # Full cleanup
make run      # Fresh start
```

**Port already in use:**
```bash
docker-compose down
make start
```