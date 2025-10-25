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

	// Collect node information
	nodeID := string(ws.node.NodeInfo().ID())
	rpcPort := extractPortFromAddress(ws.node.Config().RPC.ListenAddress)
	p2pAddress := ws.node.Config().P2P.ListenAddress

	// Get node status
	nodeStatus := "online"
	if ws.node.ConsensusReactor().WaitSync() {
		nodeStatus = "syncing"
	}
	if !ws.node.IsListening() {
		nodeStatus = "offline"
	}

	// Get peer information
	outboundPeers, inboundPeers, dialingPeers := ws.node.Switch().NumPeers()
	totalPeers := outboundPeers + inboundPeers

	// Get Tendermint status
	var latestBlockHeight int64
	var latestBlockTime string
	var catchingUp bool
	status, err := ws.cometBftRpcClient.Status(context.Background())
	if err == nil {
		latestBlockHeight = status.SyncInfo.LatestBlockHeight
		latestBlockTime = status.SyncInfo.LatestBlockTime.Format("2006-01-02 15:04:05")
		catchingUp = status.SyncInfo.CatchingUp
	}

	// Get ABCI info
	var appVersion uint64
	var lastBlockAppHash string
	abciInfo, err := ws.cometBftRpcClient.ABCIInfo(context.Background())
	if err == nil {
		appVersion = abciInfo.Response.AppVersion
		lastBlockAppHash = fmt.Sprintf("%X", abciInfo.Response.LastBlockAppHash)
	}

	uptime := time.Since(ws.startTime).Round(time.Second)

	// Determine status badge class
	statusBadge := "badge-success"
	switch nodeStatus {
	case "syncing":
		statusBadge = "badge-warning"
	case "offline":
		statusBadge = "badge-danger"
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>L1 Consensus Node - %s</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Arial, sans-serif;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            min-height: 100vh;
            padding: 40px 20px;
        }
        .container { 
            max-width: 1000px;
            margin: 0 auto;
            background: white;
            padding: 40px;
            border-radius: 16px;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
        }
        .header {
            border-bottom: 3px solid #667eea;
            padding-bottom: 20px;
            margin-bottom: 30px;
        }
        h1 { 
            color: #667eea;
            margin: 0 0 10px 0;
            font-size: 28px;
            display: flex;
            align-items: center;
            gap: 12px;
        }
        .subtitle {
            color: #666;
            font-size: 14px;
            margin-top: 8px;
        }
        .info-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
            gap: 20px;
            margin: 30px 0;
        }
        .info-card {
            background: #f8f9fa;
            padding: 20px;
            border-radius: 8px;
            border-left: 4px solid #667eea;
        }
        .info-card.consensus {
            border-left-color: #764ba2;
        }
        .info-card.network {
            border-left-color: #f093fb;
        }
        .info-card h3 {
            color: #333;
            font-size: 14px;
            text-transform: uppercase;
            letter-spacing: 1px;
            margin-bottom: 15px;
            font-weight: 600;
        }
        .info-row {
            display: flex;
            justify-content: space-between;
            padding: 8px 0;
            border-bottom: 1px solid #e9ecef;
        }
        .info-row:last-child {
            border-bottom: none;
        }
        .label { 
            font-weight: 600;
            color: #555;
            font-size: 13px;
        }
        .value { 
            color: #333;
            font-family: 'Courier New', monospace;
            font-size: 13px;
            text-align: right;
            word-break: break-all;
        }
        .value.truncate {
            max-width: 200px;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }
        .badge { 
            display: inline-block;
            padding: 4px 12px;
            border-radius: 12px;
            font-size: 11px;
            font-weight: bold;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        .badge-success { 
            background: #d4edda;
            color: #155724;
        }
        .badge-warning {
            background: #fff3cd;
            color: #856404;
        }
        .badge-danger {
            background: #f8d7da;
            color: #721c24;
        }
        .badge-info {
            background: #d1ecf1;
            color: #0c5460;
        }
        .endpoints { 
            margin-top: 40px;
        }
        .endpoints h2 {
            color: #333;
            font-size: 20px;
            margin-bottom: 20px;
            display: flex;
            align-items: center;
            gap: 10px;
        }
        .endpoint-grid {
            display: grid;
            gap: 10px;
        }
        .endpoint { 
            background: #f8f9fa;
            padding: 15px 20px;
            border-radius: 8px;
            font-family: 'Courier New', monospace;
            font-size: 13px;
            display: flex;
            align-items: center;
            gap: 15px;
            border: 1px solid #e9ecef;
            transition: all 0.2s ease;
        }
        .endpoint:hover {
            background: #e9ecef;
            border-color: #667eea;
            transform: translateX(5px);
        }
        .method { 
            font-weight: bold;
            padding: 4px 10px;
            border-radius: 4px;
            font-size: 11px;
            min-width: 50px;
            text-align: center;
        }
        .method.get {
            background: #d1ecf1;
            color: #0c5460;
        }
        .method.post {
            background: #d4edda;
            color: #155724;
        }
        .endpoint-path {
            color: #495057;
            flex: 1;
        }
        .endpoint-desc {
            color: #6c757d;
            font-size: 12px;
        }
        .stats-bar {
            display: flex;
            gap: 20px;
            margin: 20px 0;
            padding: 20px;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            border-radius: 8px;
            color: white;
        }
        .stat {
            flex: 1;
            text-align: center;
        }
        .stat-value {
            font-size: 24px;
            font-weight: bold;
            margin-bottom: 5px;
        }
        .stat-label {
            font-size: 12px;
            opacity: 0.9;
            text-transform: uppercase;
            letter-spacing: 1px;
        }
        .rpc-link {
            display: inline-block;
            color: #667eea;
            text-decoration: none;
            font-weight: 600;
            padding: 8px 16px;
            background: #f0f3ff;
            border-radius: 6px;
            margin-top: 10px;
            transition: all 0.2s ease;
        }
        .rpc-link:hover {
            background: #667eea;
            color: white;
            transform: translateY(-2px);
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>
                <span>&#x2B23;</span>
                Layer 1 - Byzantine Fault Tolerant Consensus Node
            </h1>
            <div class="subtitle">Unified BFT Consensus Layer for Multi-Shard Architecture</div>
        </div>

        <div class="stats-bar">
            <div class="stat">
                <div class="stat-value">%d</div>
                <div class="stat-label">Block Height</div>
            </div>
            <div class="stat">
                <div class="stat-value">%d</div>
                <div class="stat-label">Total Peers</div>
            </div>
            <div class="stat">
                <div class="stat-value">%s</div>
                <div class="stat-label">Uptime</div>
            </div>
        </div>
        
        <div class="info-grid">
            <div class="info-card">
                <h3>🆔 Node Information</h3>
                <div class="info-row">
                    <span class="label">Node ID:</span>
                    <span class="value truncate" title="%s">%s</span>
                </div>
                <div class="info-row">
                    <span class="label">Status:</span>
                    <span class="badge %s">%s</span>
                </div>
                <div class="info-row">
                    <span class="label">Architecture:</span>
                    <span class="value">Sharded L2 + Unified L1</span>
                </div>
                <div class="info-row">
                    <span class="label">App Version:</span>
                    <span class="value">%d</span>
                </div>
            </div>

            <div class="info-card consensus">
                <h3>⚡ Consensus Information</h3>
                <div class="info-row">
                    <span class="label">Latest Block:</span>
                    <span class="value">%d</span>
                </div>
                <div class="info-row">
                    <span class="label">Block Time:</span>
                    <span class="value">%s</span>
                </div>
                <div class="info-row">
                    <span class="label">Catching Up:</span>
                    <span class="badge %s">%t</span>
                </div>
                <div class="info-row">
                    <span class="label">App Hash:</span>
                    <span class="value truncate" title="%s">%s</span>
                </div>
            </div>

            <div class="info-card network">
                <h3>🌐 Network Information</h3>
                <div class="info-row">
                    <span class="label">P2P Address:</span>
                    <span class="value">%s</span>
                </div>
                <div class="info-row">
                    <span class="label">Outbound Peers:</span>
                    <span class="value">%d</span>
                </div>
                <div class="info-row">
                    <span class="label">Inbound Peers:</span>
                    <span class="value">%d</span>
                </div>
                <div class="info-row">
                    <span class="label">Dialing Peers:</span>
                    <span class="value">%d</span>
                </div>
            </div>
        </div>

        <div style="text-align: center; margin: 20px 0;">
            <a href="http://localhost:%s" class="rpc-link">🔗 Access RPC Interface</a>
        </div>
        
        <div class="endpoints">
            <h2>📡 API Endpoints</h2>
            <div class="endpoint-grid">
                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="endpoint-path">/l1/commit</span>
                    <span class="endpoint-desc">Receive commits from L2 shards</span>
                </div>
                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="endpoint-path">/l1/sessions/group/{group}</span>
                    <span class="endpoint-desc">Get sessions by client group</span>
                </div>
                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="endpoint-path">/l1/sessions/shard/{shard}</span>
                    <span class="endpoint-desc">Get sessions by shard ID</span>
                </div>
                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="endpoint-path">/l1/transaction/{hash}</span>
                    <span class="endpoint-desc">Get transaction details by hash</span>
                </div>
                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="endpoint-path">/l1/status</span>
                    <span class="endpoint-desc">Get L1 node status</span>
                </div>
                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="endpoint-path">/l1/shards</span>
                    <span class="endpoint-desc">Get all registered L2 shards</span>
                </div>
                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="endpoint-path">/debug</span>
                    <span class="endpoint-desc">Debug information and diagnostics</span>
                </div>
            </div>
        </div>
    </div>
</body>
</html>
	`,
		nodeID[:16]+"...", // For title
		latestBlockHeight,
		totalPeers,
		uptime,
		nodeID, nodeID[:16]+"...",
		statusBadge, nodeStatus,
		appVersion,
		latestBlockHeight,
		latestBlockTime,
		func() string {
			if catchingUp {
				return "badge-warning"
			}
			return "badge-success"
		}(),
		catchingUp,
		lastBlockAppHash, lastBlockAppHash[:16]+"...",
		p2pAddress,
		outboundPeers,
		inboundPeers,
		dialingPeers,
		rpcPort,
	)

	w.Write([]byte(html))
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
