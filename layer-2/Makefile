.PHONY: build start stop clean test logs

# Build Docker images
build:
	@echo "🔨 Building L2 Shard Docker image..."
	docker build -t l2-shard:latest .
	@echo "✅ Build complete!"

# Start L2 shards
start:
	@echo "🚀 Starting L2 Shards..."
	docker-compose up -d
	@echo "✅ L2 Shards started!"
	@echo ""
	@echo "📍 Shard A (Group A): http://localhost:6000"
	@echo "📍 Shard B (Group B): http://localhost:6001"
	@echo ""
	@echo "Run 'make logs' to view logs"

# Stop L2 shards
stop:
	@echo "🛑 Stopping L2 Shards..."
	docker-compose down
	@echo "✅ L2 Shards stopped!"

# Clean everything (including volumes)
clean:
	@echo "🧹 Cleaning up L2 Shards..."
	docker-compose down -v
	docker rmi l2-shard:latest 2>/dev/null || true
	@echo "✅ Cleanup complete!"

# View logs
logs:
	docker-compose logs -f

# Test L2 shard endpoints
test:
	@echo "🧪 Testing L2 Shard A..."
	@echo ""
	@echo "1. Testing /info endpoint..."
	@curl -s http://localhost:6000/info | jq '.' || echo "❌ Failed"
	@echo ""
	@echo "2. Creating session..."
	@curl -s -X POST http://localhost:6000/session/start \
		-H "Content-Type: application/json" \
		-d '{"operator_id":"OPR-001"}' | jq '.' || echo "❌ Failed"
	@echo ""
	@echo "✅ Basic tests complete!"

# Full workflow test
test-workflow:
	@echo "🧪 Running full workflow test on Shard A..."
	@bash test-workflow.sh

# Restart
restart: stop start

# Build and start
run: build start