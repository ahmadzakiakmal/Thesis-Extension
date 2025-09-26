package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ahmadzakiakmal/thesis-extension/layer-1/app"
	"github.com/ahmadzakiakmal/thesis-extension/layer-1/repository"
	"github.com/ahmadzakiakmal/thesis-extension/layer-1/server"
	"github.com/ahmadzakiakmal/thesis-extension/layer-1/srvreg"

	cfg "github.com/cometbft/cometbft/config"
	cmtflags "github.com/cometbft/cometbft/libs/cli/flags"
	cmtlog "github.com/cometbft/cometbft/libs/log"
	nm "github.com/cometbft/cometbft/node"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
	"github.com/cometbft/cometbft/proxy"
	cmtrpc "github.com/cometbft/cometbft/rpc/client/local"
	"github.com/dgraph-io/badger/v4"
	"github.com/spf13/viper"
)

var (
	homeDir      string
	httpPort     string
	postgresHost string
)

func init() {
	flag.StringVar(&homeDir, "cmt-home", "./node-config/l1-node", "Path to the CometBFT config directory")
	flag.StringVar(&httpPort, "http-port", "5000", "HTTP web server port")
	flag.StringVar(&postgresHost, "postgres-host", "l1-postgres0:5432", "DB host address")
}

func main() {
	// Parse command line flags
	flag.Parse()

	log.Println("=== Starting Layer 1 - Byzantine Fault Tolerant Consensus Node ===")
	log.Printf("Home Directory: %s", homeDir)
	log.Printf("HTTP Port: %s", httpPort)
	log.Printf("PostgreSQL Host: %s", postgresHost)

	// Load CometBFT configuration
	if homeDir == "" {
		homeDir = os.ExpandEnv("$HOME/.cometbft")
	}
	config := cfg.DefaultConfig()
	config.SetRoot(homeDir)
	viper.SetConfigFile(fmt.Sprintf("%s/%s", homeDir, "config/config.toml"))
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Reading config: %v", err)
	}
	if err := viper.Unmarshal(config); err != nil {
		log.Fatalf("Decoding config: %v", err)
	}
	if err := config.ValidateBasic(); err != nil {
		log.Fatalf("Invalid configuration data: %v", err)
	}

	// Connect to PostgreSQL Database
	dsn := fmt.Sprintf("postgresql://postgres:postgrespassword@%s/postgres", postgresHost)
	repository := repository.NewRepository()
	log.Printf("Connecting to PostgreSQL: %s", dsn)
	repository.ConnectDB(dsn)

	// Initialize Badger DB for blockchain storage
	badgerPath := filepath.Join(homeDir, "badger")
	db, err := badger.Open(badger.DefaultOptions(badgerPath))
	if err != nil {
		log.Fatalf("Opening badger database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Fatalf("Closing badger database: %v", err)
		}
	}()

	// Create logger
	logger := cmtlog.NewTMLogger(cmtlog.NewSyncWriter(os.Stdout))
	logger, err = cmtflags.ParseLogLevel(config.LogLevel, logger, cfg.DefaultLogLevel)
	if err != nil {
		log.Fatalf("Failed to parse log level: %v", err)
	}

	// Initialize Service Registry with L1-specific endpoints
	serviceRegistry := srvreg.NewServiceRegistry(repository, logger)
	serviceRegistry.RegisterDefaultServices()

	// Create ABCI Application
	appConfig := &app.AppConfig{
		NodeID:        filepath.Base(homeDir),
		RequiredVotes: 1,
		LogAllTxs:     true,
	}
	abciApp := app.NewABCIApplication(db, serviceRegistry, appConfig, logger, repository)

	// Load private validator
	pv := privval.LoadFilePV(
		config.PrivValidatorKeyFile(),
		config.PrivValidatorStateFile(),
	)

	// Load node key for P2P networking
	nodeKey, err := p2p.LoadNodeKey(config.NodeKeyFile())
	if err != nil {
		log.Fatalf("Failed to load node's key: %v", err)
	}

	// Initialize CometBFT node
	node, err := nm.NewNode(
		context.Background(),
		config,
		pv,
		nodeKey,
		proxy.NewLocalClientCreator(abciApp),
		nm.DefaultGenesisDocProviderFunc(config),
		cfg.DefaultDBProvider,
		nm.DefaultMetricsProvider(config.Instrumentation),
		logger,
	)
	if err != nil {
		log.Fatalf("Creating CometBFT node: %v", err)
	}

	// Set node ID in the application
	abciApp.SetNodeID(string(node.NodeInfo().ID()))
	logger.Info("L1 Node initialized", "node_id", string(node.NodeInfo().ID()))

	// Create RPC client and set up repository
	rpcClient := cmtrpc.New(node)
	repository.SetupRpcClient(rpcClient)

	// Start CometBFT node
	logger.Info("Starting CometBFT node...")
	err = node.Start()
	if err != nil {
		log.Fatalf("Starting CometBFT node: %v", err)
	}
	defer func() {
		logger.Info("Stopping CometBFT node...")
		node.Stop()
		node.Wait()
	}()

	// Start Web Server
	logger.Info("Starting L1 web server...")
	webserver, err := server.NewWebServer(abciApp, httpPort, logger, node, serviceRegistry, repository)
	if err != nil {
		log.Fatalf("Creating web server: %v", err)
	}

	err = webserver.Start()
	if err != nil {
		log.Fatalf("Starting HTTP server: %v", err)
	}

	// Display startup information
	logger.Info("=== L1 Node Successfully Started ===")
	logger.Info("Layer 1 HTTP API", "url", fmt.Sprintf("http://localhost:%s", httpPort))
	logger.Info("CometBFT RPC", "url", fmt.Sprintf("http://localhost:%s", extractPortFromAddress(config.RPC.ListenAddress)))
	logger.Info("Node ID", "id", string(node.NodeInfo().ID()))
	logger.Info("Architecture", "type", "Unified L1 for Sharded L2")

	// Display available endpoints
	logger.Info("Available L1 Endpoints:")
	logger.Info("  POST /l1/commit - Receive commits from L2 shards")
	logger.Info("  GET  /l1/sessions/group/{group} - Query sessions by client group")
	logger.Info("  GET  /l1/sessions/shard/{shard} - Query sessions by shard")
	logger.Info("  GET  /l1/transaction/{hash} - Get transaction details")
	logger.Info("  GET  /l1/status - Get L1 status")
	logger.Info("  GET  /l1/shards - Get registered shards")
	logger.Info("  GET  /debug - Debug information")

	// Wait for interrupt signal to gracefully shut down
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	logger.Info("Received shutdown signal, shutting down gracefully...")

	// Create deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Shutdown the web server
	err = webserver.Shutdown(ctx)
	if err != nil {
		logger.Error("Error shutting down HTTP web server", "err", err)
	}
	logger.Info("L1 Node gracefully stopped")
}

// extractPortFromAddress extracts the port from an address string
func extractPortFromAddress(address string) string {
	for i := len(address) - 1; i >= 0; i-- {
		if address[i] == ':' {
			return address[i+1:]
		}
	}
	return ""
}
