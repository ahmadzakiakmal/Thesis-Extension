package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ahmadzakiakmal/thesis-extension/layer-1/app"
	"github.com/ahmadzakiakmal/thesis-extension/layer-1/repository"
	"github.com/ahmadzakiakmal/thesis-extension/layer-1/srvreg"

	cmtlog "github.com/cometbft/cometbft/libs/log"
	nm "github.com/cometbft/cometbft/node"
	"github.com/cometbft/cometbft/rpc/client"
	cmthttp "github.com/cometbft/cometbft/rpc/client/http"
	cmtrpc "github.com/cometbft/cometbft/rpc/client/local"
)

// WebServer handles HTTP requests for L1
type WebServer struct {
	app                *app.Application
	httpAddr           string
	server             *http.Server
	logger             cmtlog.Logger
	node               *nm.Node
	startTime          time.Time
	serviceRegistry    *srvreg.ServiceRegistry
	cometBftHttpClient client.Client
	cometBftRpcClient  *cmtrpc.Local
	repository         *repository.Repository
}

// L1Response is the response format for L1 API calls
type L1Response struct {
	StatusCode int                 `json:"-"`
	Headers    map[string]string   `json:"-"`
	Data       interface{}         `json:"data"`
	Meta       L1TransactionStatus `json:"meta"`
	NodeID     string              `json:"node_id"`
}

// L1TransactionStatus represents the status of L1 BFT transactions
type L1TransactionStatus struct {
	TxID        string    `json:"tx_id"`
	Status      string    `json:"status"`
	BlockHeight int64     `json:"block_height"`
	ConfirmTime time.Time `json:"confirm_time"`
	ShardInfo   ShardInfo `json:"shard_info"`
}

// ShardInfo contains information about the originating shard
type ShardInfo struct {
	ShardID     string `json:"shard_id"`
	ClientGroup string `json:"client_group"`
	L2NodeID    string `json:"l2_node_id"`
}

// NewWebServer creates a new L1 web server
func NewWebServer(app *app.Application, httpPort string, logger cmtlog.Logger, node *nm.Node, serviceRegistry *srvreg.ServiceRegistry, repository *repository.Repository) (*WebServer, error) {
	mux := http.NewServeMux()

	rpcAddr := fmt.Sprintf("http://localhost:%s", extractPortFromAddress(node.Config().RPC.ListenAddress))
	logger.Info("Connecting to CometBFT RPC", "address", rpcAddr)

	// Create HTTP client for CometBFT
	cometBftHttpClient, err := cmthttp.NewWithClient(
		rpcAddr,
		&http.Client{
			Timeout: 10 * time.Second,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CometBFT client: %w", err)
	}
	err = cometBftHttpClient.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start CometBFT client: %w", err)
	}

	server := &WebServer{
		app:      app,
		httpAddr: ":" + httpPort,
		server: &http.Server{
			Addr:    ":" + httpPort,
			Handler: mux,
		},
		logger:             logger,
		node:               node,
		startTime:          time.Now(),
		serviceRegistry:    serviceRegistry,
		cometBftHttpClient: cometBftHttpClient,
		cometBftRpcClient:  cmtrpc.New(node),
		repository:         repository,
	}

	// Register routes
	mux.HandleFunc("/", server.handleRoot)
	mux.HandleFunc("/debug", server.handleDebug)
	mux.HandleFunc("/l1/", server.handleL1API)

	return server, nil
}

// Start starts the L1 web server
func (ws *WebServer) Start() error {
	ws.logger.Info("Starting L1 web server", "addr", ws.httpAddr)
	go func() {
		if err := ws.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			ws.logger.Error("L1 web server error: ", "err", err)
		}
	}()
	return nil
}

// Shutdown gracefully shuts down the web server
func (ws *WebServer) Shutdown(ctx context.Context) error {
	ws.logger.Info("Shutting down L1 web server")
	return ws.server.Shutdown(ctx)
}

// handleRoot shows L1 node information
func (ws *WebServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		JSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte("<h1>Layer 1 - Byzantine Fault Tolerant Consensus Node</h1>"))
	w.Write([]byte("<p>Node ID: " + string(ws.node.NodeInfo().ID()) + "</p>"))
	w.Write([]byte("<p>Type: BFT Consensus Layer</p>"))
	w.Write([]byte("<p>Architecture: Sharded L2 + Unified L1</p>"))

	rpcPort := extractPortFromAddress(ws.node.Config().RPC.ListenAddress)
	rpcAddrHtml := fmt.Sprintf("<p>RPC Address: <a href=\"http://localhost:%s\">http://localhost:%s</a></p>", rpcPort, rpcPort)
	w.Write([]byte(rpcAddrHtml))

	// Add API documentation
	apiDocs := `
	<h2>L1 API Endpoints</h2>
	<ul>
		<li><strong>POST /l1/commit</strong> - Receive commits from L2 shards</li>
		<li><strong>GET /l1/sessions/group/{group}</strong> - Get sessions by client group</li>
		<li><strong>GET /l1/sessions/shard/{shard}</strong> - Get sessions by shard</li>
		<li><strong>GET /l1/transaction/{hash}</strong> - Get transaction by hash</li>
		<li><strong>GET /l1/status</strong> - Get L1 status</li>
		<li><strong>GET /l1/shards</strong> - Get all registered shards</li>
	</ul>
	`
	w.Write([]byte(apiDocs))
}

// handleDebug provides L1 debugging information
func (ws *WebServer) handleDebug(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		JSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nodeStatus := "online"
	if ws.node.ConsensusReactor().WaitSync() {
		nodeStatus = "syncing"
	}
	if !ws.node.IsListening() {
		nodeStatus = "offline"
	}

	debugInfo := map[string]interface{}{
		"layer":        "L1",
		"type":         "Byzantine Fault Tolerant",
		"node_id":      string(ws.node.NodeInfo().ID()),
		"node_status":  nodeStatus,
		"p2p_address":  ws.node.Config().P2P.ListenAddress,
		"rpc_address":  ws.node.Config().RPC.ListenAddress,
		"uptime":       time.Since(ws.startTime).String(),
		"architecture": "Sharded L2 + Unified L1",
	}

	// Get consensus info
	status, err := ws.cometBftRpcClient.Status(context.Background())
	outboundPeers, inboundPeers, dialingPeers := ws.node.Switch().NumPeers()
	debugInfo["num_peers_out"] = outboundPeers
	debugInfo["num_peers_in"] = inboundPeers
	debugInfo["num_peers_dialing"] = dialingPeers

	if err != nil {
		debugInfo["consensus_error"] = err.Error()
	} else {
		debugInfo["latest_block_height"] = status.SyncInfo.LatestBlockHeight
		debugInfo["latest_block_time"] = status.SyncInfo.LatestBlockTime
		debugInfo["catching_up"] = status.SyncInfo.CatchingUp
	}

	// Add ABCI info
	abciInfo, err := ws.cometBftRpcClient.ABCIInfo(context.Background())
	if err != nil {
		debugInfo["abci_error"] = err.Error()
	} else {
		debugInfo["abci_version"] = abciInfo.Response.Version
		debugInfo["app_version"] = abciInfo.Response.AppVersion
		debugInfo["last_block_height"] = abciInfo.Response.LastBlockHeight
		debugInfo["last_block_app_hash"] = fmt.Sprintf("%X", abciInfo.Response.LastBlockAppHash)
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(debugInfo); err != nil {
		JSONError(w, "Error encoding response: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// handleL1API handles all L1 API requests
func (ws *WebServer) handleL1API(w http.ResponseWriter, r *http.Request) {
	requestID, err := generateRequestID()
	if err != nil {
		JSONError(w, "Internal Server Error", http.StatusInternalServerError)
		ws.logger.Error("Failed to generate request ID", "err", err)
		return
	}

	request, err := srvreg.ConvertHttpRequestToConsensusRequest(r, requestID)
	if err != nil {
		JSONError(w, "Failed to convert request: "+err.Error(), http.StatusUnprocessableEntity)
		ws.logger.Error("Failed to convert HTTP request", "err", err)
		return
	}

	// For L1, we don't run full consensus for every request
	// Only the /l1/commit endpoint triggers BFT consensus
	response, err := request.GenerateResponse(ws.serviceRegistry)
	if err != nil {
		JSONError(w, "Failed to generate response: "+err.Error(), http.StatusUnprocessableEntity)
		ws.logger.Error("Failed to generate response", "err", err)
		return
	}

	// Check if this was a commit request that went through consensus
	var l1Response L1Response
	if strings.Contains(r.URL.Path, "/commit") && response.StatusCode == http.StatusAccepted {
		// Parse the response to get transaction info
		var txInfo map[string]interface{}
		json.Unmarshal([]byte(response.Body), &txInfo)

		l1Response = L1Response{
			StatusCode: response.StatusCode,
			Headers:    response.Headers,
			Data:       txInfo,
			Meta: L1TransactionStatus{
				TxID:        fmt.Sprintf("%v", txInfo["tx_hash"]),
				Status:      "confirmed",
				BlockHeight: int64(txInfo["block_height"].(float64)),
				ConfirmTime: time.Now(),
				ShardInfo: ShardInfo{
					ShardID:     fmt.Sprintf("%v", txInfo["shard_id"]),
					ClientGroup: "", // Could be extracted from request if needed
					L2NodeID:    "",
				},
			},
			NodeID: string(ws.node.NodeInfo().ID()),
		}
	} else {
		// Regular response without consensus
		var responseData interface{}
		json.Unmarshal([]byte(response.Body), &responseData)

		l1Response = L1Response{
			StatusCode: response.StatusCode,
			Headers:    response.Headers,
			Data:       responseData,
			Meta: L1TransactionStatus{
				Status: "processed",
			},
			NodeID: string(ws.node.NodeInfo().ID()),
		}
	}

	// Set headers
	for key, value := range response.Headers {
		w.Header().Set(key, value)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.StatusCode)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(l1Response); err != nil {
		ws.logger.Error("Failed to encode L1 response", "err", err)
	}

	ws.logger.Info("L1 API Request Processed",
		"path", request.Path,
		"method", request.Method,
		"status", response.StatusCode,
	)
}

// Helper functions

func generateRequestID() (string, error) {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func extractPortFromAddress(address string) string {
	for i := len(address) - 1; i >= 0; i-- {
		if address[i] == ':' {
			return address[i+1:]
		}
	}
	return ""
}

func JSONError(w http.ResponseWriter, message string, statusCode int) {
	errorResponse := struct {
		Error string `json:"error"`
	}{
		Error: message,
	}
	jsonBytes, err := json.Marshal(errorResponse)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(jsonBytes)
}
