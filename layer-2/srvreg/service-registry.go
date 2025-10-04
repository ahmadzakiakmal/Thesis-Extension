package srvreg

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

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
			// Return redirect response
			return &Response{
				StatusCode: http.StatusTemporaryRedirect, // 307
				Headers:    defaultHeaders,
				Body: fmt.Sprintf(`{
					"error":"Wrong shard",
					"message":"This shard handles %s. Please connect to the correct shard.",
					"redirect_to":"%s",
					"client_group":"%s"
				}`, services.clientGroup, redirectURL, clientGroup),
			}, nil
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
