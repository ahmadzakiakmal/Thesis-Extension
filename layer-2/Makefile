.PHONY: build run start clean test logs restart

# Default configuration
NODES ?= 2

# Build Docker image
build:
	@echo "🔨 Building L2 Shard Docker image..."
	docker build -t l2-shard:latest . --no-cache
	@echo "✅ Build complete!"

# Run the L2 network (with clean and build)
run: clean build
	@echo "Setting up L2 Shard network with $(NODES) nodes..."
	@./setup-l2-network.sh -n $(NODES)
	@echo "Starting L2 network..."
	@docker-compose up -d
	@echo ""
	@echo "✅ L2 Shard Network is running!"
	@echo "   Shard Endpoints:"
	@./setup-l2-network.sh -n $(NODES) | grep "shard-" | grep "http"
	@echo ""
	@echo "🔧 Useful commands:"
	@echo "   make test    - Test all shards"
	@echo "   make logs    - View logs"
	@echo "   make clean   - Stop and cleanup"

# Start the L2 network (auto-setup config if needed)
start:
	@if [ ! -f "docker-compose.yml" ]; then \
		echo "⚙️  No configuration found. Setting up..."; \
		./setup-l2-network.sh -n $(NODES); \
	fi
	@echo "Starting L2 network..."
	@docker-compose up -d
	@echo ""
	@echo "✅ L2 Shard Network is running!"

# Stop and clean everything
clean:
	@echo "🧹 Cleaning up L2 environment..."
	@docker-compose down -v 2>/dev/null || true
	@docker rmi l2-shard:latest 2>/dev/null || true
	@echo "✅ Cleanup complete!"

# View logs
logs:
	@docker-compose logs -f

# Test L2 shard endpoints
test:
	@echo "🧪 Testing L2 Shard Endpoints..."
	@bash test-l2-endpoints.sh || echo "Create test-l2-endpoints.sh for testing"

# Restart
restart: 
	@docker-compose down
	@docker-compose up -d
	@echo "✅ L2 restarted!"

# Quick rebuild (for code changes)
rebuild:
	@echo "🔨 Rebuilding L2 image..."
	@docker-compose down
	@docker build -t l2-shard:latest .
	@docker-compose up -d
	@echo "✅ Rebuild complete!"