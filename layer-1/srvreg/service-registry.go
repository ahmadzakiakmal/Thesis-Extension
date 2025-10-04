package srvreg

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ahmadzakiakmal/thesis-extension/layer-1/repository"
	cmtlog "github.com/cometbft/cometbft/libs/log"
)

// Request represents the client's HTTP request
type Request struct {
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	RemoteAddr string            `json:"remote_addr"`
	RequestID  string            `json:"request_id"`
	Timestamp  time.Time         `json:"timestamp"`
}

// Response represents the computed response from server
type Response struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Error      string            `json:"error,omitempty"`
}

// Transaction represents a complete L1 consensus transaction
type Transaction struct {
	Request      Request  `json:"request"`
	Response     Response `json:"response"`
	OriginNodeID string   `json:"origin_node_id"`
	BlockHeight  int64    `json:"block_height,omitempty"`
}

// ServiceHandler is a function type for service handlers
type ServiceHandler func(*Request) (*Response, error)

// RouteKey uniquely identifies a route
type RouteKey struct {
	Method string
	Path   string
}

// ServiceRegistry manages all service handlers for L1
type ServiceRegistry struct {
	handlers    map[RouteKey]ServiceHandler
	exactRoutes map[RouteKey]bool
	mu          sync.RWMutex
	repository  *repository.Repository
	logger      cmtlog.Logger
}

var defaultHeaders = map[string]string{"Content-Type": "application/json"}

// NewServiceRegistry creates a new service registry for L1
func NewServiceRegistry(repository *repository.Repository, logger cmtlog.Logger) *ServiceRegistry {
	return &ServiceRegistry{
		handlers:    make(map[RouteKey]ServiceHandler),
		exactRoutes: make(map[RouteKey]bool),
		repository:  repository,
		logger:      logger,
	}
}

// GenerateRequestID generates a deterministic ID for the request
func (r *Request) GenerateRequestID() {
	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("%s-%s-%s-%s", r.Path, r.Method, r.Body, r.Timestamp)))
	r.RequestID = hex.EncodeToString(hasher.Sum(nil)[:16])
}

// RegisterHandler registers a new service handler
func (sr *ServiceRegistry) RegisterHandler(method, path string, isExactPath bool, handler ServiceHandler) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	key := RouteKey{Method: strings.ToUpper(method), Path: path}
	sr.handlers[key] = handler
	sr.exactRoutes[key] = isExactPath
}

// GetHandlerForPath finds the appropriate handler for a given path
func (sr *ServiceRegistry) GetHandlerForPath(method, path string) (ServiceHandler, bool) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	// Try exact match first
	key := RouteKey{Method: strings.ToUpper(method), Path: path}
	if handler, ok := sr.handlers[key]; ok {
		if sr.exactRoutes[key] {
			return handler, true
		}
	}

	// Try pattern matching
	for routeKey, handler := range sr.handlers {
		if routeKey.Method != strings.ToUpper(method) {
			continue
		}

		if sr.exactRoutes[routeKey] {
			continue
		}

		if matchPath(routeKey.Path, path) {
			return handler, true
		}
	}

	return nil, false
}

// matchPath does simple pattern matching for routes
func matchPath(pattern, path string) bool {
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	if len(patternParts) != len(pathParts) {
		return false
	}

	for i := range len(patternParts) {
		if strings.HasPrefix(patternParts[i], ":") {
			continue
		}
		if patternParts[i] != pathParts[i] {
			return false
		}
	}

	return true
}

// RegisterDefaultServices sets up default services for L1
func (sr *ServiceRegistry) RegisterDefaultServices() {
	// Main endpoint: Receive commits from L2 shards
	sr.RegisterHandler("POST", "/l1/commit", true, sr.ReceiveShardCommitHandler)

	// Cross-shard query endpoints
	sr.RegisterHandler("GET", "/l1/sessions/group/:group", false, sr.GetSessionsByGroupHandler)
	sr.RegisterHandler("GET", "/l1/sessions/shard/:shard", false, sr.GetSessionsByShardHandler)
	sr.RegisterHandler("GET", "/l1/transaction/:hash", false, sr.GetTransactionHandler)

	// System endpoints
	sr.RegisterHandler("GET", "/l1/status", true, sr.StatusHandler)
	sr.RegisterHandler("GET", "/l1/shards", true, sr.GetShardsHandler)
}

// ReceiveShardCommitHandler handles commits from L2 shards
func (sr *ServiceRegistry) ReceiveShardCommitHandler(req *Request) (*Response, error) {
	var commitReq repository.ShardedCommitRequest
	err := json.Unmarshal([]byte(req.Body), &commitReq)
	if err != nil {
		sr.logger.Error("Failed to parse shard commit request", "error", err.Error())
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"Invalid request format: %s"}`, err.Error()),
		}, err
	}

	// Validate required fields
	if commitReq.ShardID == "" || commitReq.SessionID == "" || commitReq.ClientGroup == "" {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       `{"error":"Missing required fields: shard_id, session_id, client_group"}`,
		}, fmt.Errorf("missing required fields")
	}

	// Process the shard commit
	transaction, repoErr := sr.repository.ReceiveShardCommit(&commitReq)
	if repoErr != nil {
		switch repoErr.Code {
		case "SHARD_NOT_FOUND":
			return &Response{
				StatusCode: http.StatusBadRequest,
				Headers:    defaultHeaders,
				Body:       fmt.Sprintf(`{"error":"%s"}`, repoErr.Detail),
			}, fmt.Errorf("shard not found: %s", repoErr.Detail)
		case "SESSION_EXISTS":
			return &Response{
				StatusCode: http.StatusConflict,
				Headers:    defaultHeaders,
				Body:       fmt.Sprintf(`{"error":"%s"}`, repoErr.Detail),
			}, fmt.Errorf("session exists: %s", repoErr.Detail)
		default:
			return &Response{
				StatusCode: http.StatusInternalServerError,
				Headers:    defaultHeaders,
				Body:       `{"error":"Internal server error"}`,
			}, fmt.Errorf("repository error: %s", repoErr.Detail)
		}
	}

	return &Response{
		StatusCode: http.StatusAccepted,
		Headers:    defaultHeaders,
		Body: fmt.Sprintf(`{
			"message": "Shard commit processed successfully",
			"tx_hash": "%s",
			"session_id": "%s",
			"shard_id": "%s",
			"block_height": %d
		}`, transaction.TxHash, transaction.SessionID, transaction.ShardID, transaction.BlockHeight),
	}, nil
}

// GetSessionsByGroupHandler retrieves sessions by client group
func (sr *ServiceRegistry) GetSessionsByGroupHandler(req *Request) (*Response, error) {
	pathParts := strings.Split(req.Path, "/")
	if len(pathParts) != 5 {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       `{"error":"Invalid path format"}`,
		}, fmt.Errorf("invalid path format")
	}

	clientGroup := pathParts[4]

	sessions, repoErr := sr.repository.GetSessionsByClientGroup(clientGroup)
	if repoErr != nil {
		return &Response{
			StatusCode: http.StatusInternalServerError,
			Headers:    defaultHeaders,
			Body:       `{"error":"Internal server error"}`,
		}, fmt.Errorf("repository error: %s", repoErr.Detail)
	}

	sessionsJSON, err := json.Marshal(sessions)
	if err != nil {
		return &Response{
			StatusCode: http.StatusInternalServerError,
			Headers:    defaultHeaders,
			Body:       `{"error":"Failed to serialize sessions"}`,
		}, err
	}

	return &Response{
		StatusCode: http.StatusOK,
		Headers:    defaultHeaders,
		Body:       string(sessionsJSON),
	}, nil
}

// GetSessionsByShardHandler retrieves sessions by shard
func (sr *ServiceRegistry) GetSessionsByShardHandler(req *Request) (*Response, error) {
	pathParts := strings.Split(req.Path, "/")
	if len(pathParts) != 5 {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       `{"error":"Invalid path format"}`,
		}, fmt.Errorf("invalid path format")
	}

	shardID := pathParts[4]

	sessions, repoErr := sr.repository.GetSessionsByShard(shardID)
	if repoErr != nil {
		return &Response{
			StatusCode: http.StatusInternalServerError,
			Headers:    defaultHeaders,
			Body:       `{"error":"Internal server error"}`,
		}, fmt.Errorf("repository error: %s", repoErr.Detail)
	}

	sessionsJSON, err := json.Marshal(sessions)
	if err != nil {
		return &Response{
			StatusCode: http.StatusInternalServerError,
			Headers:    defaultHeaders,
			Body:       `{"error":"Failed to serialize sessions"}`,
		}, err
	}

	return &Response{
		StatusCode: http.StatusOK,
		Headers:    defaultHeaders,
		Body:       string(sessionsJSON),
	}, nil
}

// GetTransactionHandler retrieves transaction by hash
func (sr *ServiceRegistry) GetTransactionHandler(req *Request) (*Response, error) {
	pathParts := strings.Split(req.Path, "/")
	if len(pathParts) != 4 {
		return &Response{
			StatusCode: http.StatusBadRequest,
			Headers:    defaultHeaders,
			Body:       `{"error":"Invalid path format"}`,
		}, fmt.Errorf("invalid path format")
	}

	txHash := pathParts[3]

	transaction, repoErr := sr.repository.GetTransactionByHash(txHash)
	if repoErr != nil {
		if repoErr.Code == "TRANSACTION_NOT_FOUND" {
			return &Response{
				StatusCode: http.StatusNotFound,
				Headers:    defaultHeaders,
				Body:       fmt.Sprintf(`{"error":"%s"}`, repoErr.Detail),
			}, fmt.Errorf("transaction not found: %s", repoErr.Detail)
		}
		return &Response{
			StatusCode: http.StatusInternalServerError,
			Headers:    defaultHeaders,
			Body:       `{"error":"Internal server error"}`,
		}, fmt.Errorf("repository error: %s", repoErr.Detail)
	}

	txJSON, err := json.Marshal(transaction)
	if err != nil {
		return &Response{
			StatusCode: http.StatusInternalServerError,
			Headers:    defaultHeaders,
			Body:       `{"error":"Failed to serialize transaction"}`,
		}, err
	}

	return &Response{
		StatusCode: http.StatusOK,
		Headers:    defaultHeaders,
		Body:       string(txJSON),
	}, nil
}

// StatusHandler provides L1 system status
func (sr *ServiceRegistry) StatusHandler(req *Request) (*Response, error) {
	status := map[string]interface{}{
		"status": "active",
		"layer":  "L1",
		"type":   "Byzantine Fault Tolerant",
		"time":   time.Now(),
	}

	statusJSON, err := json.Marshal(status)
	if err != nil {
		return &Response{
			StatusCode: http.StatusInternalServerError,
			Headers:    defaultHeaders,
			Body:       `{"error":"Failed to serialize status"}`,
		}, err
	}

	return &Response{
		StatusCode: http.StatusOK,
		Headers:    defaultHeaders,
		Body:       string(statusJSON),
	}, nil
}

// GetShardsHandler returns information about all registered shards
func (sr *ServiceRegistry) GetShardsHandler(req *Request) (*Response, error) {
	// Query shard information from the database
	shards, repoErr := sr.repository.GetAllShards()
	if repoErr != nil {
		sr.logger.Error("Failed to retrieve shards", "error", repoErr.Detail)
		return &Response{
			StatusCode: http.StatusInternalServerError,
			Headers:    defaultHeaders,
			Body:       `{"error":"Failed to retrieve shards"}`,
		}, fmt.Errorf("repository error: %s", repoErr.Detail)
	}

	// Format response
	response := map[string]interface{}{
		"shards": shards,
		"count":  len(shards),
	}

	shardsJSON, err := json.Marshal(response)
	if err != nil {
		sr.logger.Error("Failed to serialize shards", "error", err.Error())
		return &Response{
			StatusCode: http.StatusInternalServerError,
			Headers:    defaultHeaders,
			Body:       `{"error":"Failed to serialize shards"}`,
		}, err
	}

	return &Response{
		StatusCode: http.StatusOK,
		Headers:    defaultHeaders,
		Body:       string(shardsJSON),
	}, nil
}

// ConvertHttpRequestToConsensusRequest converts an http.Request to Request
func ConvertHttpRequestToConsensusRequest(r *http.Request, requestID string) (*Request, error) {
	headers := make(map[string]string)
	for name, values := range r.Header {
		if len(values) > 0 {
			headers[name] = values[0]
		}
	}

	body := ""
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		raw := strings.TrimSpace(string(bodyBytes))
		body = compactJSON(raw)
	}

	return &Request{
		Method:     r.Method,
		Path:       r.URL.Path,
		Headers:    headers,
		Body:       body,
		RemoteAddr: r.RemoteAddr,
		RequestID:  requestID,
		Timestamp:  time.Now(),
	}, nil
}

// GenerateResponse executes the request and generates a response
func (req *Request) GenerateResponse(services *ServiceRegistry) (*Response, error) {
	handler, found := services.GetHandlerForPath(req.Method, req.Path)
	if !found {
		return &Response{
			StatusCode: http.StatusNotFound,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"Service not found for %s %s"}`, req.Method, req.Path),
		}, nil
	}

	response, err := handler(req)
	return response, err
}

func compactJSON(body string) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(body)); err != nil {
		return strings.TrimSpace(body)
	}
	return buf.String()
}
