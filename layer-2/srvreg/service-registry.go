package srvreg

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ahmadzakiakmal/thesis-extension/layer-2/l1client"
	"github.com/ahmadzakiakmal/thesis-extension/layer-2/repository"
)

// Request represents an incoming HTTP request
type Request struct {
	Method  string
	Path    string
	Body    string
	Headers map[string]string
}

// Response represents an HTTP response
type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       string
}

// HandlerFunc is a function that handles a request
type HandlerFunc func(*Request) (*Response, error)

// ServiceRegistry manages all service handlers
type ServiceRegistry struct {
	handlers    map[string]map[string]HandlerFunc
	repository  *repository.Repository
	l1Client    *l1client.L1Client
	shardID     string
	clientGroup string
	logger      log.Logger
}

var defaultHeaders = map[string]string{
	"Content-Type": "application/json",
}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry(repo *repository.Repository, l1Client *l1client.L1Client, shardID, clientGroup string) *ServiceRegistry {
	return &ServiceRegistry{
		handlers:    make(map[string]map[string]HandlerFunc),
		repository:  repo,
		l1Client:    l1Client,
		shardID:     shardID,
		clientGroup: clientGroup,
		logger:      *log.New(os.Stdout, "[ServiceRegistry] ", log.LstdFlags),
	}
}

// RegisterHandler registers a handler for a specific method and path
func (sr *ServiceRegistry) RegisterHandler(method, path string, handler HandlerFunc) {
	if sr.handlers[method] == nil {
		sr.handlers[method] = make(map[string]HandlerFunc)
	}
	sr.handlers[method][path] = handler
	log.Printf("✓ Registered handler: %s %s", method, path)
}

// GetHandlerForPath finds the handler for a given method and path
func (sr *ServiceRegistry) GetHandlerForPath(method, path string) (HandlerFunc, bool) {
	methodHandlers, exists := sr.handlers[method]
	if !exists {
		return nil, false
	}

	// Try exact match first
	if handler, exists := methodHandlers[path]; exists {
		return handler, true
	}

	// Try pattern matching for paths with parameters
	for pattern, handler := range methodHandlers {
		if matchPath(pattern, path) {
			return handler, true
		}
	}

	return nil, false
}

// matchPath checks if a path matches a pattern with parameters
// It supports patterns like "/session/:id" matching "/session/123"
func matchPath(pattern, path string) bool {
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	if len(patternParts) != len(pathParts) {
		return false
	}

	for i := 0; i < len(patternParts); i++ {
		if strings.HasPrefix(patternParts[i], ":") {
			// This is a parameter, it matches anything
			continue
		}
		if patternParts[i] != pathParts[i] {
			return false
		}
	}

	return true
}

// RegisterDefaultServices sets up all default endpoints
func (sr *ServiceRegistry) RegisterDefaultServices() {
	log.Println("Registering L2 shard services...")

	// Session endpoints
	sr.RegisterHandler("POST", "/session/start", sr.CreateSessionHandler)
	sr.RegisterHandler("GET", "/session/:id/scan", sr.ScanPackageHandler)
	sr.RegisterHandler("POST", "/session/:id/validate", sr.ValidatePackageHandler)
	sr.RegisterHandler("POST", "/session/:id/qc", sr.QualityCheckHandler)
	sr.RegisterHandler("POST", "/session/:id/label", sr.LabelPackageHandler)
	sr.RegisterHandler("POST", "/session/:id/commit", sr.CommitSessionHandler)

	// Info endpoints
	sr.RegisterHandler("GET", "/info", sr.InfoHandler)

	log.Println("✓ All services registered")
}

// GenerateResponse executes the request and generates a response
func (req *Request) GenerateResponse(services *ServiceRegistry) (*Response, error) {
	// Check client group header and redirect if needed
	clientGroup := req.Headers["X-Client-Group"]
	if clientGroup != "" {
		shouldHandle, redirectURL := services.CheckShardAndRedirect(clientGroup)
		if !shouldHandle {
			// Forward to correct shard instead of returning redirect
			return services.ForwardToCorrectShard(req, redirectURL)
		}
	}

	// Continue with normal handler routing
	handler, found := services.GetHandlerForPath(req.Method, req.Path)

	if !found {
		return &Response{
			StatusCode: 404,
			Headers:    defaultHeaders,
			Body:       fmt.Sprintf(`{"error":"Service not found for %s %s"}`, req.Method, req.Path),
		}, nil
	}

	response, err := handler(req)
	return response, err
}

// CheckShardAndRedirect checks if the client group belongs to this shard
// Returns (shouldHandle, redirectURL)
// CheckShardAndRedirect checks if the client group belongs to this shard
// Returns (shouldHandle, redirectURL)
func (sr *ServiceRegistry) CheckShardAndRedirect(clientGroup string) (bool, string) {
	// If client group matches this shard, handle it
	if clientGroup == sr.clientGroup {
		return true, ""
	}

	// Client group doesn't match - find the correct shard
	shard, found := sr.l1Client.GetShardByClientGroup(clientGroup)
	if !found {
		// Unknown client group - let this shard handle it (will likely fail later)
		sr.logger.Printf("⚠️  Unknown client group: %s", clientGroup)
		return true, ""
	}

	// Return redirect URL
	sr.logger.Printf("↪️  Redirecting client_group=%s to shard=%s at %s", clientGroup, shard.ShardID, shard.L2Endpoint)

	return false, shard.L2Endpoint
}

// ForwardToCorrectShard forwards the request to the correct shard and measures time
func (sr *ServiceRegistry) ForwardToCorrectShard(req *Request, targetURL string) (*Response, error) {
	startTime := time.Now()

	// Construct the full URL
	fullURL := fmt.Sprintf("%s%s", targetURL, req.Path)

	sr.logger.Printf("🔄 Forwarding request to correct shard: %s %s", req.Method, fullURL)

	// Create HTTP request
	httpReq, err := http.NewRequest(req.Method, fullURL, bytes.NewBufferString(req.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to create forward request: %w", err)
	}

	// Copy headers from original request
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Make the request
	client := &http.Client{Timeout: 30 * time.Second}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to forward request: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read forward response: %w", err)
	}

	// Measure time
	forwardLatency := time.Since(startTime).Milliseconds()

	sr.logger.Printf("✅ Cross-shard request completed in %d ms", forwardLatency)

	// Return the response from the correct shard
	return &Response{
		StatusCode: httpResp.StatusCode,
		Headers:    defaultHeaders,
		Body:       string(bodyBytes),
	}, nil
}
