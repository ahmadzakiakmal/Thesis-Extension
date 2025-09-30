package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/ahmadzakiakmal/thesis-extension/layer-2/srvreg"
)

// WebServer handles HTTP requests for L2 shard
type WebServer struct {
	httpAddr        string
	server          *http.Server
	serviceRegistry *srvreg.ServiceRegistry
	startTime       time.Time
	shardID         string
	clientGroup     string
}

// NewWebServer creates a new L2 web server
func NewWebServer(httpPort string, serviceRegistry *srvreg.ServiceRegistry, shardID, clientGroup string) *WebServer {
	mux := http.NewServeMux()

	ws := &WebServer{
		httpAddr: ":" + httpPort,
		server: &http.Server{
			Addr:    ":" + httpPort,
			Handler: mux,
		},
		serviceRegistry: serviceRegistry,
		startTime:       time.Now(),
		shardID:         shardID,
		clientGroup:     clientGroup,
	}

	// Register routes
	mux.HandleFunc("/", ws.handleRoot)
	mux.HandleFunc("/info", ws.handleInfo)
	mux.HandleFunc("/session/", ws.handleSession)

	return ws
}

// Start starts the L2 web server
func (ws *WebServer) Start() error {
	log.Printf("🚀 Starting L2 Shard Web Server")
	log.Printf("   Shard ID: %s", ws.shardID)
	log.Printf("   Client Group: %s", ws.clientGroup)
	log.Printf("   Address: %s", ws.httpAddr)

	go func() {
		if err := ws.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("❌ L2 web server error: %v", err)
		}
	}()

	log.Println("✓ L2 web server started successfully")
	return nil
}

// Shutdown gracefully shuts down the web server
func (ws *WebServer) Shutdown(ctx context.Context) error {
	log.Println("Shutting down L2 web server...")
	return ws.server.Shutdown(ctx)
}

// handleRoot shows shard information
func (ws *WebServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uptime := time.Since(ws.startTime).Round(time.Second)

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>L2 Shard - %s</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background: #f5f5f5; }
        .container { background: white; padding: 30px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        h1 { color: #2c5aa0; margin-top: 0; }
        .info { margin: 20px 0; }
        .label { font-weight: bold; color: #555; }
        .value { color: #333; margin-left: 10px; }
        .badge { display: inline-block; padding: 4px 12px; border-radius: 12px; font-size: 12px; font-weight: bold; }
        .badge-success { background: #d4edda; color: #155724; }
        .endpoints { margin-top: 30px; }
        .endpoint { background: #f8f9fa; padding: 10px; margin: 8px 0; border-radius: 4px; font-family: monospace; }
        .method { font-weight: bold; color: #007bff; margin-right: 10px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔷 Layer 2 Shard Node</h1>
        
        <div class="info">
            <div><span class="label">Shard ID:</span><span class="value">%s</span></div>
            <div><span class="label">Client Group:</span><span class="value">%s</span></div>
            <div><span class="label">Status:</span><span class="badge badge-success">Active</span></div>
            <div><span class="label">Uptime:</span><span class="value">%s</span></div>
        </div>
        
        <div class="endpoints">
            <h3>Available Endpoints:</h3>
            <div class="endpoint"><span class="method">GET</span>/info - Shard information</div>
            <div class="endpoint"><span class="method">POST</span>/session/start - Create new session</div>
            <div class="endpoint"><span class="method">GET</span>/session/:id/scan - Scan package</div>
            <div class="endpoint"><span class="method">POST</span>/session/:id/validate - Validate package</div>
            <div class="endpoint"><span class="method">POST</span>/session/:id/qc - Quality check</div>
            <div class="endpoint"><span class="method">POST</span>/session/:id/label - Create shipping label</div>
            <div class="endpoint"><span class="method">POST</span>/session/:id/commit - Commit to L1</div>
        </div>
    </div>
</body>
</html>
	`, ws.shardID, ws.shardID, ws.clientGroup, uptime)

	w.Write([]byte(html))
}

// handleInfo returns shard information as JSON
func (ws *WebServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	req := &srvreg.Request{
		Method: r.Method,
		Path:   r.URL.Path,
		Body:   "",
	}

	response, err := req.GenerateResponse(ws.serviceRegistry)
	if err != nil {
		log.Printf("Error generating response: %v", err)
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	writeResponse(w, response)
}

// handleSession handles all session-related endpoints
func (ws *WebServer) handleSession(w http.ResponseWriter, r *http.Request) {
	// Read request body
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		jsonError(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Create request object
	req := &srvreg.Request{
		Method: r.Method,
		Path:   r.URL.Path,
		Body:   string(bodyBytes),
	}

	// Generate response through service registry
	response, err := req.GenerateResponse(ws.serviceRegistry)
	if err != nil {
		log.Printf("Error generating response: %v", err)
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	writeResponse(w, response)
}

// writeResponse writes a Response to http.ResponseWriter
func writeResponse(w http.ResponseWriter, resp *srvreg.Response) {
	// Set headers
	for key, value := range resp.Headers {
		w.Header().Set(key, value)
	}

	// Set status code
	w.WriteHeader(resp.StatusCode)

	// Write body
	w.Write([]byte(resp.Body))
}

// jsonError writes a JSON error response
func jsonError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResp := map[string]string{
		"error": message,
	}
	json.NewEncoder(w).Encode(errorResp)
}
