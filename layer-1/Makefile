.PHONY: build run start clean test monitor

# Default configuration
NODES ?= 4

# Build everything we need
build:
	@echo "Building L1 binary..."
	@mkdir -p ./build
	@CGO_ENABLED=0 go build -o ./build/bin
	@chmod +x ./build/bin
	@echo "Building Docker image..."
	@docker build -t l1-node:latest .
	@echo "Build complete!"

# Run the L1 network (with clean and build)
run: clean build
	@echo "Setting up L1 BFT network with $(NODES) nodes..."
	@./setup-l1-network.sh -n $(NODES)
	@echo "Starting L1 network..."
	@docker-compose up -d
	@echo ""
	@echo "✅ L1 Network is running!"
	@echo "   API Endpoints:"
	@for i in $$(seq 0 $$(($$(echo $(NODES)) - 1))); do \
		port=$$((5000 + $$i)); \
		echo "   - Node $$i: http://localhost:$$port"; \
	done
	@echo ""
	@echo "🔧 Useful commands:"
	@echo "   make test    - Test all endpoints"
	@echo "   make monitor - Monitor network health"
	@echo "   make clean   - Stop and cleanup"

# Start the L1 network (auto-setup config if needed)
start:
	@if [ ! -d "node-config/node0" ]; then \
		echo "⚙️  No configuration found. Setting up..."; \
		./setup-l1-network.sh -n $(NODES); \
	fi
	@echo "Starting L1 network..."
	@docker-compose up -d
	@echo ""
	@echo "✅ L1 Network is running!"
	@echo "   API Endpoints:"
	@for i in $$(seq 0 $$(($$(echo $(NODES)) - 1))); do \
		port=$$((5000 + $$i)); \
		echo "   - Node $$i: http://localhost:$$port"; \
	done

# Stop and clean everything
clean:
	@echo "Cleaning up L1 environment..."
	@docker-compose down -v 2>/dev/null || true
	@sudo rm -rf node-config 2>/dev/null || true
	@echo "Cleanup complete!"

# Test the L1 endpoints
test:
	@echo "Testing L1 endpoints..."
	@./test-l1-endpoints.sh

# Monitor the network
monitor:
	@./monitor-l1-network.sh

# Quick single node for debugging
debug: clean
	@echo "Running single L1 node for debugging..."
	@mkdir -p ./build
	@go build -o ./build/bin
	@./setup-l1-network.sh -n 1
	@echo "Starting debug node..."
	@./build/bin --cmt-home=./node-config/node0 --http-port=5000 --postgres-host=localhost:5432