package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ahmadzakiakmal/thesis-extension/layer-2/config"
	"github.com/ahmadzakiakmal/thesis-extension/layer-2/l1client"
	"github.com/ahmadzakiakmal/thesis-extension/layer-2/repository"
	"github.com/ahmadzakiakmal/thesis-extension/layer-2/server"
	"github.com/ahmadzakiakmal/thesis-extension/layer-2/srvreg"
)

func main() {
	// Parse command line flags (optional, for overriding env vars)
	configFile := flag.String("config", "", "Config file path (optional)")
	flag.Parse()

	if *configFile != "" {
		log.Printf("Config file support not implemented yet, using environment variables")
	}

	log.Println("===========================================")
	log.Println("   L2 Shard Node - Starting Up")
	log.Println("===========================================")

	// Load configuration
	cfg := config.LoadConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("❌ Configuration validation failed: %v", err)
	}

	log.Printf("✓ Configuration loaded")
	log.Printf("   Shard ID: %s", cfg.ShardID)
	log.Printf("   Client Group: %s", cfg.ClientGroup)
	log.Printf("   L2 Node ID: %s", cfg.L2NodeID)
	log.Printf("   HTTP Port: %s", cfg.HTTPPort)
	log.Printf("   L1 Endpoint: %s", cfg.L1Endpoint)
	log.Printf("   Database: %s:%s/%s", cfg.DatabaseHost, cfg.DatabasePort, cfg.DatabaseName)

	// Initialize repository
	log.Println("\n📦 Initializing database...")
	repo := repository.NewRepository()
	if err := repo.ConnectDB(cfg.GetDSN()); err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}

	// Initialize L1 client
	log.Println("\n🔗 Initializing L1 client...")
	l1Client := l1client.NewL1Client(cfg.L1Endpoint, cfg.ShardID, cfg.L2NodeID)

	// Test L1 connection
	if err := l1Client.HealthCheck(); err != nil {
		log.Printf("⚠️  Warning: L1 health check failed: %v", err)
		log.Println("   L2 will start anyway, but commits to L1 will fail until L1 is available")
	} else {
		log.Println("✓ L1 connection verified")
	}

	// Load shard information from L1
	log.Println("📋 Loading shard registry from L1...")
	if err := l1Client.LoadShards(); err != nil {
		log.Printf("⚠️  Warning: Failed to load shards: %v", err)
		log.Println("   Redirect functionality will not be available")
	} else {
		log.Println("✓ Shard registry loaded")
	}

	// Initialize service registry
	log.Println("\nSetting up service registry...")
	serviceRegistry := srvreg.NewServiceRegistry(repo, l1Client, cfg.ShardID, cfg.ClientGroup)
	serviceRegistry.RegisterDefaultServices()

	// Initialize web server
	log.Println("\nStarting web server...")
	webServer := server.NewWebServer(cfg.HTTPPort, serviceRegistry, cfg.ShardID, cfg.ClientGroup)
	if err := webServer.Start(); err != nil {
		log.Fatalf("❌ Failed to start web server: %v", err)
	}

	log.Println("\n===========================================")
	log.Printf("   L2 Shard Node Ready!")
	log.Printf("   Shard: %s (Group: %s)", cfg.ShardID, cfg.ClientGroup)
	log.Printf("   Listening on: http://localhost:%s", cfg.HTTPPort)
	log.Println("===========================================")
	log.Println("")

	// Wait for interrupt signal to gracefully shut down
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("\n🛑 Shutdown signal received, gracefully shutting down...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown web server
	if err := webServer.Shutdown(ctx); err != nil {
		log.Printf("❌ Error during server shutdown: %v", err)
	}

	log.Println("✓ L2 Shard Node stopped")
	log.Println("Goodbye! 👋")
}
