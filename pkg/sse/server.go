package sse

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
	httpclient "swagger-docs-mcp/pkg/http"
	"swagger-docs-mcp/pkg/server"
	"swagger-docs-mcp/pkg/swagger"
	"swagger-docs-mcp/pkg/types"
	"swagger-docs-mcp/pkg/utils"
)

// SSEServer implements Server-Sent Events for Swagger tools
type SSEServer struct {
	config            *types.ResolvedConfig
	logger            *utils.Logger
	scanner           *swagger.Scanner
	parser            *swagger.Parser
	generator         *swagger.ToolGenerator
	promptGenerator   *swagger.PromptGenerator
	resourceGenerator *swagger.ResourceGenerator
	toolRegistry      *server.ToolRegistry
	promptRegistry    *server.PromptRegistry
	resourceRegistry  *server.ResourceRegistry
	httpClient        *httpclient.Client
	server            *http.Server
	clients           map[string]*SSEClient
	clientsMutex      sync.RWMutex
	shutdown          chan struct{}
	wg                sync.WaitGroup
}

// SSEClient represents a connected SSE client
type SSEClient struct {
	ID       string
	Writer   http.ResponseWriter
	Flusher  http.Flusher
	Request  *http.Request
	Context  context.Context
	Cancel   context.CancelFunc
	LastSeen time.Time
}

// SSEEvent represents an event to be sent to clients
type SSEEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
	ID   string      `json:"id,omitempty"`
}

// ToolListEvent is sent when tools are available
type ToolListEvent struct {
	Tools []types.MCPTool `json:"tools"`
}

// ToolExecutionEvent is sent when a tool is executed
type ToolExecutionEvent struct {
	ToolName   string                  `json:"toolName"`
	Arguments  map[string]interface{}  `json:"arguments"`
	Result     types.MCPCallToolResult `json:"result"`
	ExecutedAt time.Time               `json:"executedAt"`
}

// ErrorEvent is sent when an error occurs
type ErrorEvent struct {
	Message string `json:"message"`
	Code    int    `json:"code,omitempty"`
}

// NewSSEServer creates a new SSE server
func NewSSEServer(config *types.ResolvedConfig, logger *utils.Logger) *SSEServer {
	scanner := swagger.NewScanner(logger)
	parser := swagger.NewParser(logger)
	generator := swagger.NewToolGeneratorWithConfig(logger, &config.ToolGeneration)
	promptGenerator := swagger.NewPromptGenerator(logger, &config.Prompts)
	resourceGenerator := swagger.NewResourceGenerator(logger, &config.Resources)
	toolRegistry := server.NewToolRegistry()
	promptRegistry := server.NewPromptRegistry()
	resourceRegistry := server.NewResourceRegistry()
	httpClient := httpclient.NewClient(config, logger)

	return &SSEServer{
		config:            config,
		logger:            logger.Child("sse-server"),
		scanner:           scanner,
		parser:            parser,
		generator:         generator,
		promptGenerator:   promptGenerator,
		resourceGenerator: resourceGenerator,
		toolRegistry:      toolRegistry,
		promptRegistry:    promptRegistry,
		resourceRegistry:  resourceRegistry,
		httpClient:        httpClient,
		clients:           make(map[string]*SSEClient),
		shutdown:          make(chan struct{}),
	}
}

// Start starts the SSE server
func (s *SSEServer) Start(ctx context.Context) error {
	s.logger.Info("Starting SSE server", 
		zap.String("name", s.config.Name), 
		zap.String("version", s.config.Version),
		zap.Duration("timeout", s.config.Server.Timeout))

	// Initialize tools first
	if err := s.initializeTools(ctx); err != nil {
		return fmt.Errorf("failed to initialize tools: %w", err)
	}

	// Setup HTTP router
	router := mux.NewRouter()
	s.setupRoutes(router)

	// Create HTTP server
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Server.Port),
		Handler:      s.addMiddleware(router),
		ReadTimeout:  s.config.Server.Timeout,
		WriteTimeout: s.config.Server.Timeout,
		IdleTimeout:  s.config.Server.Timeout * 2,
	}

	// Start cleanup routine
	s.wg.Add(1)
	go s.cleanupClients()

	// Start server
	s.logger.Info("SSE server listening", zap.String("address", s.server.Addr))
	
	serverErr := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait for shutdown signal or server error
	select {
	case <-ctx.Done():
		s.logger.Info("Context cancelled, shutting down")
	case <-s.shutdown:
		s.logger.Info("Shutdown signal received")
	case err := <-serverErr:
		return fmt.Errorf("server error: %w", err)
	}

	return s.stop()
}

// Stop stops the SSE server
func (s *SSEServer) Stop() {
	select {
	case <-s.shutdown:
		return
	default:
		close(s.shutdown)
	}
}

// stop performs the actual shutdown
func (s *SSEServer) stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.Error("Error shutting down server", zap.Error(err))
	}

	// Close all SSE clients
	s.clientsMutex.Lock()
	for _, client := range s.clients {
		client.Cancel()
	}
	s.clients = make(map[string]*SSEClient)
	s.clientsMutex.Unlock()

	// Wait for cleanup routine
	close(s.shutdown)
	s.wg.Wait()

	s.logger.Info("SSE server stopped")
	return nil
}

// setupRoutes sets up HTTP routes
func (s *SSEServer) setupRoutes(router *mux.Router) {
	// Health check endpoints
	router.HandleFunc("/health", s.handleHealth).Methods("GET")
	router.HandleFunc("/healthz", s.handleHealth).Methods("GET")
	router.HandleFunc("/ready", s.handleHealth).Methods("GET")
	router.HandleFunc("/readyz", s.handleHealth).Methods("GET")
	
	// SSE endpoints
	router.HandleFunc("/events", s.handleSSE).Methods("GET")
	
	// Tool management
	router.HandleFunc("/tools", s.handleListTools).Methods("GET")
	router.HandleFunc("/tools/{name}/execute", s.handleExecuteTool).Methods("POST")
	
	// Prompt management
	router.HandleFunc("/prompts", s.handleListPrompts).Methods("GET")
	router.HandleFunc("/prompts/{name}", s.handleGetPrompt).Methods("GET", "POST")
	
	// Resource management
	router.HandleFunc("/resources", s.handleListResources).Methods("GET")
	router.HandleFunc("/resources/read", s.handleReadResource).Methods("POST")
	
	// Configuration
	router.HandleFunc("/config", s.handleGetConfig).Methods("GET")
	
	// Version information
	router.HandleFunc("/version", s.handleGetVersion).Methods("GET")
	
	// Root endpoint (must be last to avoid conflicts)
	router.HandleFunc("/", s.handleRoot).Methods("GET")
	router.HandleFunc("/mcp", s.handleRoot).Methods("GET")
}

// addMiddleware adds middleware to the router
func (s *SSEServer) addMiddleware(handler http.Handler) http.Handler {
	// CORS middleware
	corsHandler := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
			
			if r.Method == "OPTIONS" {
				return
			}
			
			next.ServeHTTP(w, r)
		})
	}

	// Logging middleware
	loggingHandler := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			s.logger.Info("HTTP request",
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
				zap.String("remote_addr", r.RemoteAddr),
				zap.Duration("duration", time.Since(start)))
		})
	}

	return corsHandler(loggingHandler(handler))
}

// cleanupClients removes inactive clients
func (s *SSEServer) cleanupClients() {
	defer s.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.shutdown:
			return
		case <-ticker.C:
			s.clientsMutex.Lock()
			now := time.Now()
			for id, client := range s.clients {
				if now.Sub(client.LastSeen) > 2*time.Minute {
					s.logger.Debug("Removing inactive client", zap.String("clientID", id))
					client.Cancel()
					delete(s.clients, id)
				}
			}
			s.clientsMutex.Unlock()
		}
	}
}

// createTempHTTPClient creates a temporary HTTP client with custom configuration
func (s *SSEServer) createTempHTTPClient(config *types.ResolvedConfig) *httpclient.Client {
	return httpclient.NewClient(config, s.logger)
}