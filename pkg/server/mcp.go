package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"go.uber.org/zap"
	"swagger-docs-mcp/pkg/http"
	"swagger-docs-mcp/pkg/swagger"
	"swagger-docs-mcp/pkg/types"
	"swagger-docs-mcp/pkg/utils"
)

// MCPServer implements the Model Context Protocol server
type MCPServer struct {
	config       *types.ResolvedConfig
	logger       *utils.Logger
	scanner      *swagger.Scanner
	parser       *swagger.Parser
	generator    *swagger.ToolGenerator
	toolRegistry *ToolRegistry
	httpClient   *http.Client
	stdin        io.Reader
	stdout       io.Writer
	initialized  bool
	shutdown     chan struct{}
	wg           sync.WaitGroup
}

// NewMCPServer creates a new MCP server
func NewMCPServer(config *types.ResolvedConfig, logger *utils.Logger) *MCPServer {
	scanner := swagger.NewScanner(logger)
	parser := swagger.NewParser(logger)
	generator := swagger.NewToolGenerator(logger)
	toolRegistry := NewToolRegistry()
	httpClient := http.NewClient(config, logger)

	return &MCPServer{
		config:       config,
		logger:       logger.Child("mcp-server"),
		scanner:      scanner,
		parser:       parser,
		generator:    generator,
		toolRegistry: toolRegistry,
		httpClient:   httpClient,
		stdin:        os.Stdin,
		stdout:       os.Stdout,
		shutdown:     make(chan struct{}),
	}
}

// Start starts the MCP server
func (s *MCPServer) Start(ctx context.Context) error {
	s.logger.Info("Starting MCP server", zap.String("name", s.config.Name), zap.String("version", s.config.Version))

	// Note: Tool initialization is now deferred until the first MCP initialize request
	// This prevents issues with the MCP protocol handshake

	// Start message handling loop
	s.wg.Add(1)
	go s.handleMessages(ctx)

	// Wait for shutdown
	select {
	case <-ctx.Done():
		s.logger.Info("Context cancelled, shutting down")
	case <-s.shutdown:
		s.logger.Info("Shutdown signal received")
	}

	close(s.shutdown)
	s.wg.Wait()

	s.logger.Info("MCP server stopped")
	return nil
}

// Stop stops the MCP server
func (s *MCPServer) Stop() {
	select {
	case <-s.shutdown:
		// Already shutting down
		return
	default:
		close(s.shutdown)
	}
}

// initializeTools initializes swagger documents and generates tools
func (s *MCPServer) initializeTools(ctx context.Context) error {
	s.logger.Info("Initializing swagger documents and tools")

	// Scan swagger documents
	scanResult, err := s.scanner.ScanPathsAndURLs(
		s.config.SwaggerPaths,
		s.config.SwaggerURLs,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to scan swagger documents: %w", err)
	}

	s.logger.Info("Scan complete",
		zap.Int("totalFiles", scanResult.Stats.TotalFiles),
		zap.Int("validDocuments", scanResult.Stats.ValidDocuments),
		zap.Int("errors", scanResult.Stats.Errors),
		zap.String("scanTime", scanResult.Stats.ScanTime.String()))

	// Apply filters
	documents := scanResult.Documents

	// Filter by package IDs
	if len(s.config.PackageIDs) > 0 {
		documents = s.scanner.FilterDocumentsByPackageIDs(documents, s.config.PackageIDs)
		s.logger.Debug("Filtered by package IDs", zap.Int("documentsRemaining", len(documents)))
	}

	// Filter by TWC filters
	if s.config.TWCFilters != nil {
		documents = s.scanner.FilterDocumentsByTWCFilters(documents, s.config.TWCFilters)
		s.logger.Debug("Filtered by TWC filters", zap.Int("documentsRemaining", len(documents)))
	}

	// Filter by dynamic filters
	if len(s.config.DynamicFilters) > 0 {
		documents = s.scanner.FilterDocumentsByDynamicFilters(documents, s.config.DynamicFilters)
		s.logger.Debug("Filtered by dynamic filters", zap.Int("documentsRemaining", len(documents)))
	}

	// Parse documents and generate tools
	toolCount := 0
	for _, docInfo := range documents {
		var parsedDoc *types.SwaggerDocument
		var err error

		// Use appropriate parsing method based on whether content is available
		if docInfo.IsRemote && len(docInfo.Content) > 0 {
			parsedDoc, err = s.parser.ParseDocumentWithContent(&docInfo)
		} else {
			parsedDoc, err = s.parser.ParseDocument(docInfo.FilePath)
		}

		if err != nil {
			s.logger.Error("Failed to parse document",
				zap.Error(err),
				zap.String("filePath", docInfo.FilePath),
				zap.String("title", docInfo.Title),
				zap.Int("contentSize", len(docInfo.Content)),
				zap.Bool("isRemote", docInfo.IsRemote))
			continue
		}

		// Generate tools from parsed document
		tools, err := s.generator.GenerateToolsFromDocument(parsedDoc, &docInfo)
		if err != nil {
			s.logger.Error("Failed to generate tools from document",
				zap.Error(err),
				zap.String("filePath", docInfo.FilePath),
				zap.String("title", docInfo.Title),
				zap.Int("pathCount", getPathCount(parsedDoc)),
				zap.String("version", docInfo.Version))
			continue
		}

		// Register tools
		for _, tool := range tools {
			if err := s.toolRegistry.RegisterTool(tool); err != nil {
				s.logger.Error("Failed to register tool",
					zap.Error(err),
					zap.String("toolName", tool.Name),
					zap.String("document", docInfo.Title),
					zap.String("method", tool.Endpoint.Method),
					zap.String("path", tool.Endpoint.Path),
					zap.String("operationID", tool.Endpoint.OperationID))
				// Continue processing other tools even if one fails
			} else {
				toolCount++
				s.logger.Debug("Successfully registered tool",
					zap.String("toolName", tool.Name),
					zap.String("method", tool.Endpoint.Method),
					zap.String("path", tool.Endpoint.Path),
					zap.String("document", docInfo.Title),
					zap.String("version", docInfo.Version))
			}
		}

		// Check max tools limit
		if s.config.Server.MaxTools > 0 && toolCount >= s.config.Server.MaxTools {
			s.logger.Warn("Reached maximum tool limit, stopping tool generation", zap.Int("maxTools", s.config.Server.MaxTools))
			break
		}
	}

	s.logger.Info("Tool initialization complete",
		zap.Int("documentsProcessed", len(documents)),
		zap.Int("toolsGenerated", toolCount),
		zap.Int("toolsRegistered", s.toolRegistry.GetToolCount()))

	return nil
}

// handleMessages handles incoming MCP messages
func (s *MCPServer) handleMessages(ctx context.Context) {
	defer s.wg.Done()

	scanner := bufio.NewScanner(s.stdin)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdown:
			return
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		s.logger.Debug("Received message", zap.String("message", line))

		// Parse the JSON-RPC message
		var request types.MCPRequest
		if err := json.Unmarshal([]byte(line), &request); err != nil {
			s.logger.Error("Failed to parse JSON-RPC message", zap.Error(err), zap.String("rawMessage", line))
			s.sendErrorResponse(nil, -32700, "Parse error", nil)
			continue
		}

		// Handle the request
		if err := s.handleRequest(&request); err != nil {
			s.logger.Error("Failed to handle request", zap.Error(err), zap.String("method", request.Method))
		}
	}

	if err := scanner.Err(); err != nil {
		s.logger.Error("Error reading from stdin", zap.Error(err))
	}
}

// handleRequest handles a specific MCP request
func (s *MCPServer) handleRequest(request *types.MCPRequest) error {
	switch request.Method {
	case "initialize":
		return s.handleInitialize(request)
	case "initialized", "notifications/initialized":
		return s.handleInitialized(request)
	case "tools/list":
		return s.handleListTools(request)
	case "tools/call":
		return s.handleCallTool(request)
	case "prompts/list":
		return s.handleListPrompts(request)
	case "prompts/get":
		return s.handleGetPrompt(request)
	case "resources/list":
		return s.handleListResources(request)
	case "resources/read":
		return s.handleReadResource(request)
	default:
		// Check if this is a notification (no ID field)
		if request.ID == nil {
			// Notifications should not return error responses
			s.logger.Debug("Ignoring unknown notification", zap.String("method", request.Method))
			return nil
		}
		return s.sendErrorResponse(request.ID, -32601, "Method not found", nil)
	}
}

// handleInitialize handles the initialize request
func (s *MCPServer) handleInitialize(request *types.MCPRequest) error {
	s.logger.Debug("Handling initialize request")

	capabilities := types.MCPCapabilities{
		Tools: &types.MCPToolsCapability{
			ListChanged: true,
		},
	}

	// Add prompts capability if enabled
	if s.config.Prompts.Enabled {
		capabilities.Prompts = &types.MCPPromptsCapability{
			ListChanged: true,
		}
	}

	// Add resources capability if enabled
	if s.config.Resources.Enabled {
		capabilities.Resources = &types.MCPResourcesCapability{
			Subscribe:   false,
			ListChanged: true,
		}
	}

	// Add logging capability
	capabilities.Logging = &types.MCPLoggingCapability{}

	result := types.MCPInitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities:    capabilities,
		ServerInfo: types.MCPServerInfo{
			Name:    s.config.Name,
			Version: s.config.Version,
		},
	}

	return s.sendResponse(request.ID, result)
}

// handleInitialized handles the initialized notification
func (s *MCPServer) handleInitialized(request *types.MCPRequest) error {
	s.logger.Debug("Handling initialized notification")
	s.initialized = true

	// Now that MCP is initialized, trigger tool initialization in background
	go func() {
		ctx := context.Background()
		if err := s.initializeTools(ctx); err != nil {
			s.logger.Error("Failed to initialize tools after MCP handshake", zap.Error(err))
		}
	}()

	return nil
}

// handleListTools handles the tools/list request
func (s *MCPServer) handleListTools(request *types.MCPRequest) error {
	s.logger.Debug("Handling tools/list request")

	tools := s.toolRegistry.GetAllTools()
	mcpTools := make([]types.MCPTool, len(tools))

	for i, tool := range tools {
		mcpTools[i] = types.MCPTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		}
	}

	result := types.MCPListToolsResult{
		Tools: mcpTools,
	}

	s.logger.Debug("Returning tools", zap.Int("count", len(mcpTools)))
	return s.sendResponse(request.ID, result)
}

// handleCallTool handles the tools/call request
func (s *MCPServer) handleCallTool(request *types.MCPRequest) error {
	s.logger.Debug("Handling tools/call request")

	// Parse parameters
	paramsBytes, err := json.Marshal(request.Params)
	if err != nil {
		return s.sendErrorResponse(request.ID, -32602, "Invalid params", nil)
	}

	var params types.MCPCallToolParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		return s.sendErrorResponse(request.ID, -32602, "Invalid params", nil)
	}

	// Get the tool
	tool := s.toolRegistry.GetTool(params.Name)
	if tool == nil {
		return s.sendErrorResponse(request.ID, -32601, "Tool not found", nil)
	}

	s.logger.Debug("Executing tool", zap.String("name", params.Name), zap.Any("arguments", params.Arguments))

	// Execute the tool
	result, err := s.executeAPICall(tool, params.Arguments)
	if err != nil {
		s.logger.Error("Tool execution failed", zap.Error(err), zap.String("toolName", params.Name))
		errorContent := types.MCPContent{
			Type: "text",
			Text: fmt.Sprintf("Error executing tool: %s", err.Error()),
		}
		return s.sendResponse(request.ID, types.MCPCallToolResult{
			Content: []types.MCPContent{errorContent},
			IsError: true,
		})
	}

	return s.sendResponse(request.ID, result)
}

// handleListPrompts handles the prompts/list request
func (s *MCPServer) handleListPrompts(request *types.MCPRequest) error {
	s.logger.Debug("Handling prompts/list request")
	// TODO: Implement prompts functionality
	return s.sendResponse(request.ID, map[string]interface{}{"prompts": []interface{}{}})
}

// handleGetPrompt handles the prompts/get request
func (s *MCPServer) handleGetPrompt(request *types.MCPRequest) error {
	s.logger.Debug("Handling prompts/get request")
	// TODO: Implement prompts functionality
	return s.sendErrorResponse(request.ID, -32601, "Prompt not found", nil)
}

// handleListResources handles the resources/list request
func (s *MCPServer) handleListResources(request *types.MCPRequest) error {
	s.logger.Debug("Handling resources/list request")
	// TODO: Implement resources functionality
	return s.sendResponse(request.ID, map[string]interface{}{"resources": []interface{}{}})
}

// handleReadResource handles the resources/read request
func (s *MCPServer) handleReadResource(request *types.MCPRequest) error {
	s.logger.Debug("Handling resources/read request")
	// TODO: Implement resources functionality
	return s.sendErrorResponse(request.ID, -32601, "Resource not found", nil)
}

// executeAPICall executes an API call using the HTTP client
func (s *MCPServer) executeAPICall(tool *types.GeneratedTool, arguments map[string]interface{}) (types.MCPCallToolResult, error) {
	// Execute the HTTP request
	response, err := s.httpClient.ExecuteRequest(tool.Endpoint, arguments)
	if err != nil {
		return types.MCPCallToolResult{}, err
	}

	// Convert response to MCP content
	content := types.MCPContent{
		Type: "text",
		Text: string(response.Body),
	}

	if response.Headers["Content-Type"] != "" {
		content.MimeType = response.Headers["Content-Type"]
	}

	return types.MCPCallToolResult{
		Content: []types.MCPContent{content},
		IsError: response.StatusCode >= 400,
	}, nil
}

// sendResponse sends a JSON-RPC response
func (s *MCPServer) sendResponse(id interface{}, result interface{}) error {
	response := types.MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	return s.sendMessage(response)
}

// sendErrorResponse sends a JSON-RPC error response
func (s *MCPServer) sendErrorResponse(id interface{}, code int, message string, data interface{}) error {
	response := types.MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &types.MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	return s.sendMessage(response)
}

// sendMessage sends a message to stdout
func (s *MCPServer) sendMessage(message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	s.logger.Debug("Sending message", zap.String("message", string(data)))

	data = append(data, '\n')

	if _, err := s.stdout.Write(data); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// getPathCount safely gets the number of paths in a swagger document
func getPathCount(document *types.SwaggerDocument) int {
	if document.Paths == nil {
		return 0
	}
	return len(document.Paths)
}
