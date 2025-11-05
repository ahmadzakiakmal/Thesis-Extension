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

// withCORS wraps a handler, adds CORS headers, and answers preflight globally.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// If you are NOT using credentials (cookies/Authorization in "include" mode), "*" is fine:
		if origin == "" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			// If you might use credentials later, prefer echoing specific origin:
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		// Vary so caches don't mix responses across origins/methods/headers
		w.Header().Set("Vary", "Origin, Access-Control-Request-Method, Access-Control-Request-Headers")

		// Methods you support
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

		// Echo requested headers (fallback to common ones)
		reqHdrs := r.Header.Get("Access-Control-Request-Headers")
		if reqHdrs == "" {
			reqHdrs = "Content-Type, Authorization"
		}
		w.Header().Set("Access-Control-Allow-Headers", reqHdrs)

		// If you plan to send cookies/credentials from the browser, uncomment these two lines
		// and DO NOT use "*" for Allow-Origin; you must echo the real origin.
		// w.Header().Set("Access-Control-Allow-Credentials", "true")
		// w.Header().Set("Access-Control-Allow-Origin", origin)

		// Preflight: return 200 OK and stop.
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// NewWebServer creates a new L2 web server
func NewWebServer(httpPort string, serviceRegistry *srvreg.ServiceRegistry, shardID, clientGroup string) *WebServer {
	mux := http.NewServeMux()

	ws := &WebServer{
		httpAddr: ":" + httpPort,
		server: &http.Server{
			Addr:    ":" + httpPort,
			Handler: withCORS(mux),
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
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>L2 Shard - %s</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Arial, sans-serif;
            background: linear-gradient(135deg, #11998e 0%%, #38ef7d 100%%);
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
            border-bottom: 3px solid #11998e;
            padding-bottom: 20px;
            margin-bottom: 30px;
        }
        h1 { 
            color: #11998e;
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
            border-left: 4px solid #43e97b;
        }
        .info-card.shard {
            border-left-color: #38f9d7;
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
            border-color: #43e97b;
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
            background: linear-gradient(135deg, #11998e 0%%, #38ef7d 100%%);
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
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>
                <span>&#x2B23;</span>
                Layer 2 Shard Node
            </h1>
            <div class="subtitle">Independent Shard for Client Group-Based Data Processing</div>
        </div>

        <div class="stats-bar">
            <div class="stat">
                <div class="stat-value">%s</div>
                <div class="stat-label">Shard ID</div>
            </div>
            <div class="stat">
                <div class="stat-value">%s</div>
                <div class="stat-label">Client Group</div>
            </div>
            <div class="stat">
                <div class="stat-value">%s</div>
                <div class="stat-label">Uptime</div>
            </div>
        </div>
        
        <div class="info-grid">
            <div class="info-card">
                <h3>&#x1F4CB; Shard Information</h3>
                <div class="info-row">
                    <span class="label">Shard ID:</span>
                    <span class="value">%s</span>
                </div>
                <div class="info-row">
                    <span class="label">Client Group:</span>
                    <span class="value">%s</span>
                </div>
                <div class="info-row">
                    <span class="label">Status:</span>
                    <span class="badge badge-success">Active</span>
                </div>
                <div class="info-row">
                    <span class="label">Architecture:</span>
                    <span class="value">Sharded L2</span>
                </div>
            </div>

            <div class="info-card shard">
                <h3>&#x1F310; Operational Info</h3>
                <div class="info-row">
                    <span class="label">Layer:</span>
                    <span class="value">L2</span>
                </div>
                <div class="info-row">
                    <span class="label">Type:</span>
                    <span class="value">Independent Shard</span>
                </div>
                <div class="info-row">
                    <span class="label">Uptime:</span>
                    <span class="value">%s</span>
                </div>
                <div class="info-row">
                    <span class="label">Consensus:</span>
                    <span class="badge badge-info">None (Shard)</span>
                </div>
            </div>
        </div>
        
        <div class="endpoints">
            <h2>&#x1F4E1; API Endpoints</h2>
            <div class="endpoint-grid">
                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="endpoint-path">/info</span>
                    <span class="endpoint-desc">Shard information</span>
                </div>
                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="endpoint-path">/session/start</span>
                    <span class="endpoint-desc">Create new session</span>
                </div>
                <div class="endpoint">
                    <span class="method get">GET</span>
                    <span class="endpoint-path">/session/:id/scan</span>
                    <span class="endpoint-desc">Scan package</span>
                </div>
                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="endpoint-path">/session/:id/validate</span>
                    <span class="endpoint-desc">Validate package</span>
                </div>
                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="endpoint-path">/session/:id/qc</span>
                    <span class="endpoint-desc">Quality check</span>
                </div>
                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="endpoint-path">/session/:id/label</span>
                    <span class="endpoint-desc">Create shipping label</span>
                </div>
                <div class="endpoint">
                    <span class="method post">POST</span>
                    <span class="endpoint-path">/session/:id/commit</span>
                    <span class="endpoint-desc">Commit to L1</span>
                </div>
            </div>
        </div>
    </div>
</body>
</html>
	`,
		ws.shardID,     // Title
		ws.shardID,     // Stats bar - Shard ID
		ws.clientGroup, // Stats bar - Client Group
		uptime,         // Stats bar - Uptime
		ws.shardID,     // Info card - Shard ID
		ws.clientGroup, // Info card - Client Group
		uptime,         // Operational info - Uptime
	)

	w.Write([]byte(html))
}

// handleInfo returns shard information as JSON
func (ws *WebServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	req := &srvreg.Request{
		Method:  r.Method,
		Path:    r.URL.Path,
		Body:    "",
		Headers: convertHeaders(r.Header),
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
		Method:  r.Method,
		Path:    r.URL.Path,
		Body:    string(bodyBytes),
		Headers: convertHeaders(r.Header),
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

// convertHeaders converts http.Header to map[string]string
func convertHeaders(httpHeaders http.Header) map[string]string {
	headers := make(map[string]string)
	for key, values := range httpHeaders {
		if len(values) > 0 {
			headers[key] = values[0] // Take first value if multiple
		}
	}
	return headers
}
