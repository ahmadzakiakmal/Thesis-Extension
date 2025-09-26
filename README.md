# Layer 1 - BFT Consensus for Sharded L2

A simplified Byzantine Fault Tolerant consensus layer that handles commits from multiple L2 shards.

## Quick Start

```bash
# Full setup (clean + build + start)
make run

# Quick restart (no build)
make run-only  

# Just build everything
make build

# Test all endpoints
make test

# Monitor network health
make monitor

# Stop everything
make clean

# See all commands
make help
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

After `make run`, your L1 nodes are available at:
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

- `make run` - Full setup: clean + build + start
- `make run-only` - Quick restart without building  
- `make build` - Build binary and Docker image only
- `make test` - Test L1 endpoints  
- `make monitor` - Check network health
- `make clean` - Stop and cleanup
- `make debug` - Run single node for debugging
- `make help` - Show all available commands

**Pro Tips:**
- Use `make run` for first setup or after code changes
- Use `make run-only` for quick restarts (much faster!)
- Use `make run NODES=7` for different node counts

## Requirements

- Go 1.24+
- Docker & Docker Compose
- jq (for JSON processing in scripts)

That's it! 🎉