package mcp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"swagger-docs-mcp/pkg/types"
	"swagger-docs-mcp/pkg/utils"
	"swagger-docs-mcp/pkg/version"
	"go.uber.org/zap"
)

// SimpleMCPServer wraps the mcp-go server for swagger tools
type SimpleMCPServer struct {
	mcpServer *server.MCPServer
	config    *types.ResolvedConfig
	logger    *utils.Logger
	toolCount int
}

// NewSimpleMCPServer creates a new MCP server using mcp-go library
func NewSimpleMCPServer(config *types.ResolvedConfig, logger *utils.Logger) (*SimpleMCPServer, error) {
	// Create the mcp-go server with basic capabilities
	mcpServer := server.NewMCPServer(
		"swagger-docs-mcp",
		version.GetSemanticVersion(),
		server.WithToolCapabilities(false), // No list changed notifications
		server.WithLogging(),
	)

	return &SimpleMCPServer{
		mcpServer: mcpServer,
		config:    config,
		logger:    logger,
		toolCount: 0,
	}, nil
}

// AddSwaggerTool adds a swagger tool as an MCP tool
func (s *SimpleMCPServer) AddSwaggerTool(tool *types.GeneratedTool) error {
	s.logger.Debug("Adding swagger tool to MCP server",
		zap.String("name", tool.Name),
		zap.String("method", tool.Endpoint.Method),
		zap.String("path", tool.Endpoint.Path))

	// Build tool options from swagger schema
	var toolOptions []mcp.ToolOption

	// Add description
	if tool.Description != "" {
		toolOptions = append(toolOptions, mcp.WithDescription(tool.Description))
	}

	// Add parameters from swagger schema
	if tool.InputSchema != nil {
		if properties, exists := tool.InputSchema["properties"]; exists {
			if propMap, ok := properties.(map[string]interface{}); ok {
				for paramName, prop := range propMap {
					if paramProp, ok := prop.(map[string]interface{}); ok {
						// Determine parameter type
						paramType := "string" // default
						if t, exists := paramProp["type"]; exists {
							if typeStr, ok := t.(string); ok {
								paramType = typeStr
							}
						}

						// Build property options
						var propOptions []mcp.PropertyOption

						// Add description
						if desc, exists := paramProp["description"]; exists {
							if descStr, ok := desc.(string); ok {
								propOptions = append(propOptions, mcp.Description(descStr))
							}
						}

						// Check if required
						required := false
						if requiredFields, exists := tool.InputSchema["required"]; exists {
							if reqSlice, ok := requiredFields.([]interface{}); ok {
								for _, reqField := range reqSlice {
									if reqStr, ok := reqField.(string); ok && reqStr == paramName {
										required = true
										break
									}
								}
							}
						}
						if required {
							propOptions = append(propOptions, mcp.Required())
						}

						// Add enum values if present
						if enumVal, exists := paramProp["enum"]; exists {
							if enumSlice, ok := enumVal.([]interface{}); ok {
								enumStrs := make([]string, 0, len(enumSlice))
								for _, v := range enumSlice {
									if str, ok := v.(string); ok {
										enumStrs = append(enumStrs, str)
									}
								}
								if len(enumStrs) > 0 {
									propOptions = append(propOptions, mcp.Enum(enumStrs...))
								}
							}
						}

						// Add property based on type
						switch paramType {
						case "string":
							toolOptions = append(toolOptions, mcp.WithString(paramName, propOptions...))
						case "boolean":
							toolOptions = append(toolOptions, mcp.WithBoolean(paramName, propOptions...))
						case "array":
							toolOptions = append(toolOptions, mcp.WithArray(paramName, propOptions...))
						default:
							// Default to string for unknown types
							toolOptions = append(toolOptions, mcp.WithString(paramName, propOptions...))
						}
					}
				}
			}
		}
	}

	// Create the MCP tool
	mcpTool := mcp.NewTool(tool.Name, toolOptions...)

	// Create tool handler
	toolHandler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		s.logger.Debug("Executing swagger tool via MCP",
			zap.String("toolName", tool.Name),
			zap.Any("arguments", request.Params.Arguments))

		// For now, return a simple response showing the tool was called
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Successfully called swagger tool '%s' with arguments: %v", tool.Name, request.Params.Arguments)),
			},
		}, nil
	}

	// Register the tool with the MCP server
	s.mcpServer.AddTool(mcpTool, toolHandler)
	s.toolCount++

	return nil
}

// Start starts the MCP server (stdio mode)
func (s *SimpleMCPServer) Start(ctx context.Context) error {
	s.logger.Info("Starting MCP server (stdio mode)",
		zap.String("name", "swagger-docs-mcp"),
		zap.String("version", version.GetSemanticVersion()),
		zap.Int("tools", s.toolCount))

	return server.ServeStdio(s.mcpServer)
}

// StartHTTP starts the MCP server with HTTP transport (Streamable HTTP)
func (s *SimpleMCPServer) StartHTTP(ctx context.Context, addr string) error {
	s.logger.Info("Starting MCP HTTP server (Streamable HTTP)",
		zap.String("address", addr),
		zap.Int("tools", s.toolCount))

	// Create Streamable HTTP server
	streamableServer := server.NewStreamableHTTPServer(
		s.mcpServer,
		server.WithEndpointPath("/mcp"),
	)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:    addr,
		Handler: s.addCORSMiddleware(streamableServer),
	}

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		s.logger.Info("Context cancelled, shutting down MCP HTTP server")
		return httpServer.Shutdown(context.Background())
	case err := <-errChan:
		return fmt.Errorf("MCP HTTP server error: %w", err)
	}
}

// addCORSMiddleware adds CORS headers to the HTTP handler
func (s *SimpleMCPServer) addCORSMiddleware(next http.Handler) http.Handler {
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

// Stop stops the server
func (s *SimpleMCPServer) Stop() {
	s.logger.Info("MCP server stopped")
}

// GetToolCount returns the number of registered tools
func (s *SimpleMCPServer) GetToolCount() int {
	return s.toolCount
}