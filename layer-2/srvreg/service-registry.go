package srvreg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/ahmadzakiakmal/thesis-extension/layer-2/l1client"
	"github.com/ahmadzakiakmal/thesis-extension/layer-2/repository"
)

// Request represents an incoming HTTP request
type Request struct {
	Method string
	Path   string
	Body   string
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

// compactJSON removes whitespace from JSON
func compactJSON(body string) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(body)); err != nil {
		return strings.TrimSpace(body)
	}
	return buf.String()
}
