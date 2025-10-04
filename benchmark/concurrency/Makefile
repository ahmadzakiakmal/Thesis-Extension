.PHONY: build run clean test-quick test-medium test-heavy

# Build the benchmark
build:
	@echo "🔨 Building concurrency benchmark..."
	@mkdir -p bin
	go build -o bin/benchmark .
	@echo "✅ Build complete!"

# Quick test (10 workers, 10 seconds)
test-quick: build
	@echo "🚀 Running quick concurrency test..."
	./bin/benchmark -l1=4 -l2=1 -workers=10 -duration=10 -port=7000

# Medium test (50 workers, 30 seconds)
test-medium: build
	@echo "🚀 Running medium concurrency test..."
	./bin/benchmark -l1=4 -l2=1 -workers=50 -duration=30 -port=7000

# Heavy test (100 workers, 60 seconds)
test-heavy: build
	@echo "🚀 Running heavy concurrency test..."
	./bin/benchmark -l1=4 -l2=1 -workers=100 -duration=60 -port=7000

# Custom run with parameters
run: build
	@echo "Usage: make run L1=4 L2=1 WORKERS=10 DURATION=30 PORT=7000"
	./bin/benchmark -l1=$(L1) -l2=$(L2) -workers=$(WORKERS) -duration=$(DURATION) -port=$(PORT)

# Run full test suite (varying workers)
test-suite: build
	@echo "📊 Running concurrency test suite..."
	@echo "\n=== Test 1: 10 workers ==="
	./bin/benchmark -l1=4 -l2=1 -workers=10 -duration=30 -port=7000
	@sleep 5
	@echo "\n=== Test 2: 25 workers ==="
	./bin/benchmark -l1=4 -l2=1 -workers=25 -duration=30 -port=7000
	@sleep 5
	@echo "\n=== Test 3: 50 workers ==="
	./bin/benchmark -l1=4 -l2=1 -workers=50 -duration=30 -port=7000
	@sleep 5
	@echo "\n=== Test 4: 100 workers ==="
	./bin/benchmark -l1=4 -l2=1 -workers=100 -duration=30 -port=7000
	@echo "\n✅ Test suite complete!"

# Clean build artifacts
clean:
	@echo "🧹 Cleaning..."
	rm -rf bin
	@echo "✅ Clean complete!"