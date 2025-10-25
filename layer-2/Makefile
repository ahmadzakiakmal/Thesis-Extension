.PHONY: build run start clean test logs restart rebuild

# Default configuration
NODES ?= 2

# Build Docker image
build:
	@echo "🔨 Building L2 Shard Docker image..."
	@sudo docker build -t l2-shard:latest . --no-cache
	@echo "✅ Build complete!"

# Run the L2 network (with clean and build)
run: clean build
	@echo "Setting up L2 Shard network with $(NODES) nodes..."
	@./setup-l2-network.sh -n $(NODES)
	@echo "Starting L2 network..."
	@sudo docker-compose up -d
	@echo ""
	@echo "✅ L2 Shard Network is running!"
	@echo "   Shard Endpoints:"
	@./setup-l2-network.sh -n $(NODES) | grep "shard-" | grep "http"
	@echo ""
	@echo "🔧 Useful commands:"
	@echo "   make test    - Test all shards"
	@echo "   make logs    - View logs"
	@echo "   make clean   - Stop and cleanup"
	@echo "   make rebuild - Quick rebuild after code changes"

# Start the L2 network (auto-setup config if needed)
start:
	@./setup-l2-network.sh -n $(NODES)
	@echo "Starting L2 network... (n = $(NODES))"
	@sudo docker-compose up -d
	@echo ""
	@echo "✅ L2 Shard Network is running!"

# Stop and clean everything
clean:
	@echo "🧹 Cleaning up L2 environment..."
	@sudo docker-compose down -v 2>/dev/null || true
	@sudo docker images | grep "layer-2-" | awk '{print $$3}' | xargs -r sudo docker rmi -f 2>/dev/null || true
	@sudo docker rmi -f l2-shard:latest 2>/dev/null || true
	@echo "✅ Cleanup complete!"

# View logs
logs:
	@sudo docker-compose logs -f

# Test L2 shard endpoints
test:
	@echo "🧪 Testing L2 Shard Endpoints..."
	@bash test-l2-endpoints.sh || echo "Create test-l2-endpoints.sh for testing"

# Restart containers without rebuilding
restart: 
	@echo "🔄 Restarting L2 containers..."
	@sudo docker-compose down
	@sudo docker-compose up -d
	@echo "✅ L2 restarted!"

# Quick rebuild (for code changes) - with --no-cache to ensure changes are picked up
rebuild:
	@echo "🔨 Rebuilding L2 image with fresh code..."
	@sudo docker-compose down
	@sudo docker build --no-cache -t l2-shard:latest .
	@sudo docker-compose up -d
	@echo "✅ Rebuild complete! Your changes should now be live."
	@echo "   Check the UI at your shard endpoints"