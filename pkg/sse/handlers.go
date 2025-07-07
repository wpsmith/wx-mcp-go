package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"swagger-docs-mcp/pkg/types"
)

// handleHealth handles health check requests
func (s *SSEServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   s.config.Version,
		"tools":     s.toolRegistry.GetToolCount(),
		"clients":   len(s.clients),
	}
	
	json.NewEncoder(w).Encode(health)
}

// handleSSE handles Server-Sent Events connections
func (s *SSEServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Check if client supports SSE
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create client context
	ctx, cancel := context.WithCancel(r.Context())
	clientID := uuid.New().String()

	client := &SSEClient{
		ID:       clientID,
		Writer:   w,
		Flusher:  flusher,
		Request:  r,
		Context:  ctx,
		Cancel:   cancel,
		LastSeen: time.Now(),
	}

	// Register client
	s.clientsMutex.Lock()
	s.clients[clientID] = client
	s.clientsMutex.Unlock()

	s.logger.Info("New SSE client connected", zap.String("clientID", clientID), zap.String("remoteAddr", r.RemoteAddr))

	// Send initial events
	s.sendEventToClient(client, SSEEvent{
		Type: "connected",
		Data: map[string]interface{}{
			"clientID": clientID,
			"serverInfo": map[string]interface{}{
				"name":    s.config.Name,
				"version": s.config.Version,
			},
		},
		ID: uuid.New().String(),
	})

	// Send current tools list
	tools := s.toolRegistry.GetAllTools()
	mcpTools := make([]types.MCPTool, len(tools))
	for i, tool := range tools {
		mcpTools[i] = types.MCPTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		}
	}

	s.sendEventToClient(client, SSEEvent{
		Type: "tools",
		Data: ToolListEvent{Tools: mcpTools},
		ID:   uuid.New().String(),
	})

	// Keep connection alive and handle client disconnect
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("SSE client disconnected", zap.String("clientID", clientID))
			s.clientsMutex.Lock()
			delete(s.clients, clientID)
			s.clientsMutex.Unlock()
			return
		case <-heartbeat.C:
			client.LastSeen = time.Now()
			s.sendEventToClient(client, SSEEvent{
				Type: "heartbeat",
				Data: map[string]interface{}{"timestamp": time.Now().UTC()},
				ID:   uuid.New().String(),
			})
		}
	}
}

// handleListTools handles GET /tools requests with dynamic filtering support
func (s *SSEServer) handleListTools(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse query parameters for dynamic filtering
	queryParams := r.URL.Query()
	
	// Extract filtering parameters from query string
	packageIDs := parseCommaSeparated(queryParams.Get("package-ids"))
	twcDomains := parseCommaSeparated(queryParams.Get("twc-domains"))
	twcPortfolios := parseCommaSeparated(queryParams.Get("twc-portfolios"))
	twcGeographies := parseCommaSeparated(queryParams.Get("twc-geographies"))
	customFilters := parseCommaSeparated(queryParams.Get("filter-custom"))
	
	s.logger.Debug("Dynamic filtering requested",
		zap.Strings("packageIDs", packageIDs),
		zap.Strings("twcDomains", twcDomains),
		zap.Strings("twcPortfolios", twcPortfolios),
		zap.Strings("twcGeographies", twcGeographies),
		zap.Strings("customFilters", customFilters))

	// Get all tools first
	allTools := s.toolRegistry.GetAllTools()
	
	// Apply dynamic filtering if any filters are specified
	filteredTools := allTools
	if len(packageIDs) > 0 || len(twcDomains) > 0 || len(twcPortfolios) > 0 || len(twcGeographies) > 0 || len(customFilters) > 0 {
		filteredTools = s.applyDynamicFilters(allTools, packageIDs, twcDomains, twcPortfolios, twcGeographies, customFilters)
		s.logger.Debug("Applied dynamic filters", 
			zap.Int("originalCount", len(allTools)), 
			zap.Int("filteredCount", len(filteredTools)))
	}

	// Convert to MCP format
	mcpTools := make([]types.MCPTool, len(filteredTools))
	for i, tool := range filteredTools {
		mcpTools[i] = types.MCPTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		}
	}

	result := map[string]interface{}{
		"tools": mcpTools,
		"count": len(mcpTools),
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

// handleExecuteTool handles POST /tools/{name}/execute requests
func (s *SSEServer) handleExecuteTool(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	toolName := vars["name"]

	w.Header().Set("Content-Type", "application/json")

	// Get the tool
	tool := s.toolRegistry.GetTool(toolName)
	if tool == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Tool not found",
			"code":  404,
		})
		return
	}

	// Parse request body
	var request struct {
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.logger.Error("Failed to decode request body", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Invalid request body",
			"code":  400,
		})
		return
	}

	s.logger.Debug("Executing tool", zap.String("name", toolName), zap.Any("arguments", request.Arguments))

	// Check if API key is provided in arguments for dynamic override
	var apiKey string
	if argAPIKey, exists := request.Arguments["apiKey"]; exists {
		if keyStr, ok := argAPIKey.(string); ok && keyStr != "" {
			apiKey = keyStr
			s.logger.Debug("Using API key from request arguments")
			// Remove apiKey from arguments to prevent it from being passed as a parameter
			// unless it's actually defined as a parameter in the swagger spec
			delete(request.Arguments, "apiKey")
		}
	}

	// Execute the tool with dynamic API key if provided
	result, err := s.executeAPICallWithAPIKey(tool, request.Arguments, apiKey)
	if err != nil {
		s.logger.Error("Tool execution failed", zap.Error(err), zap.String("toolName", toolName))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("Error executing tool: %s", err.Error()),
			"code":  500,
		})
		return
	}

	// Send execution event to all SSE clients
	executionEvent := SSEEvent{
		Type: "tool_execution",
		Data: ToolExecutionEvent{
			ToolName:   toolName,
			Arguments:  request.Arguments,
			Result:     result,
			ExecutedAt: time.Now().UTC(),
		},
		ID: uuid.New().String(),
	}
	s.broadcastEvent(executionEvent)

	// Return result
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

// handleGetConfig handles GET /config requests
func (s *SSEServer) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	config := map[string]interface{}{
		"name":         s.config.Name,
		"version":      s.config.Version,
		"debug":        s.config.Debug,
		"toolCount":    s.toolRegistry.GetToolCount(),
		"clientCount":  len(s.clients),
		"swaggerPaths": s.config.SwaggerPaths,
		"swaggerURLs":  s.config.SwaggerURLs,
		"server": map[string]interface{}{
			"port":     s.config.Server.Port,
			"timeout":  s.config.Server.Timeout.String(),
			"maxTools": s.config.Server.MaxTools,
		},
		"http": map[string]interface{}{
			"timeout":   s.config.HTTP.Timeout.String(),
			"retries":   s.config.HTTP.Retries,
			"userAgent": s.config.HTTP.UserAgent,
		},
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(config)
}

// sendEventToClient sends an SSE event to a specific client
func (s *SSEServer) sendEventToClient(client *SSEClient, event SSEEvent) {
	select {
	case <-client.Context.Done():
		return
	default:
	}

	data, err := json.Marshal(event.Data)
	if err != nil {
		s.logger.Error("Failed to marshal event data", zap.Error(err))
		return
	}

	// Format as SSE
	var message string
	if event.ID != "" {
		message += fmt.Sprintf("id: %s\n", event.ID)
	}
	message += fmt.Sprintf("event: %s\n", event.Type)
	message += fmt.Sprintf("data: %s\n\n", string(data))

	// Write to client
	if _, err := client.Writer.Write([]byte(message)); err != nil {
		s.logger.Debug("Failed to write to SSE client", zap.Error(err), zap.String("clientID", client.ID))
		client.Cancel()
		return
	}

	client.Flusher.Flush()
}

// broadcastEvent sends an SSE event to all connected clients
func (s *SSEServer) broadcastEvent(event SSEEvent) {
	s.clientsMutex.RLock()
	defer s.clientsMutex.RUnlock()

	for _, client := range s.clients {
		go s.sendEventToClient(client, event)
	}
}

// executeAPICall executes an API call using the HTTP client
func (s *SSEServer) executeAPICall(tool *types.GeneratedTool, arguments map[string]interface{}) (types.MCPCallToolResult, error) {
	return s.executeAPICallWithAPIKey(tool, arguments, "")
}

// executeAPICallWithAPIKey executes an API call with optional dynamic API key override
func (s *SSEServer) executeAPICallWithAPIKey(tool *types.GeneratedTool, arguments map[string]interface{}, apiKey string) (types.MCPCallToolResult, error) {
	// Create a temporary HTTP client with overridden API key if provided
	httpClient := s.httpClient
	if apiKey != "" {
		// Clone the config and override the API key
		tempConfig := *s.config
		tempConfig.Auth.APIKey = apiKey
		
		// Create a temporary HTTP client with the new config
		httpClient = s.createTempHTTPClient(&tempConfig)
		s.logger.Debug("Created temporary HTTP client with dynamic API key")
	}

	// Execute the HTTP request
	response, err := httpClient.ExecuteRequest(tool.Endpoint, arguments)
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


// parseCommaSeparated parses a comma-separated string into a slice
func parseCommaSeparated(value string) []string {
	if value == "" {
		return []string{}
	}
	
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	
	return result
}

// applyDynamicFilters applies runtime filtering to tools based on query parameters
func (s *SSEServer) applyDynamicFilters(tools []*types.GeneratedTool, packageIDs, twcDomains, twcPortfolios, twcGeographies, customFilters []string) []*types.GeneratedTool {
	var filtered []*types.GeneratedTool
	
	for _, tool := range tools {
		// Check if tool matches any of the filtering criteria
		if s.matchesTool(tool, packageIDs, twcDomains, twcPortfolios, twcGeographies, customFilters) {
			filtered = append(filtered, tool)
		}
	}
	
	return filtered
}

// matchesTool checks if a tool matches the filtering criteria
func (s *SSEServer) matchesTool(tool *types.GeneratedTool, packageIDs, twcDomains, twcPortfolios, twcGeographies, customFilters []string) bool {
	if tool.DocumentInfo == nil {
		s.logger.Debug("Tool has no document info, skipping filters", zap.String("toolName", tool.Name))
		return len(packageIDs) == 0 && len(twcDomains) == 0 && len(twcPortfolios) == 0 && len(twcGeographies) == 0 && len(customFilters) == 0
	}
	
	// Filter by package IDs
	if len(packageIDs) > 0 {
		if !hasAnyMatch(packageIDs, tool.DocumentInfo.PackageIDs) {
			return false
		}
	}
	
	// Filter by TWC domains  
	if len(twcDomains) > 0 {
		if !hasAnyMatch(twcDomains, tool.DocumentInfo.TwcDomain) {
			return false
		}
	}
	
	// Filter by TWC portfolios
	if len(twcPortfolios) > 0 {
		if !hasAnyMatch(twcPortfolios, tool.DocumentInfo.TwcDomainPortfolio) {
			return false
		}
	}
	
	// Filter by TWC geographies
	if len(twcGeographies) > 0 {
		if !hasAnyMatch(twcGeographies, tool.DocumentInfo.TwcGeography) {
			return false
		}
	}
	
	// Filter by custom filters (check title, description, endpoint tags)
	if len(customFilters) > 0 {
		matched := false
		for _, filter := range customFilters {
			if strings.Contains(strings.ToLower(tool.DocumentInfo.Title), strings.ToLower(filter)) ||
			   strings.Contains(strings.ToLower(tool.Description), strings.ToLower(filter)) {
				matched = true
				break
			}
			
			// Check endpoint tags if available
			if tool.Endpoint != nil && containsInSlice(tool.Endpoint.Tags, filter) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	
	return true
}

// contains checks if a slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// hasAnyMatch checks if any item in the first slice matches any item in the second slice
func hasAnyMatch(searchItems []string, targetItems []string) bool {
	for _, searchItem := range searchItems {
		for _, targetItem := range targetItems {
			if searchItem == targetItem {
				return true
			}
		}
	}
	return false
}

// containsInSlice checks if any string in the slice contains the search term (case-insensitive)
func containsInSlice(slice []string, searchTerm string) bool {
	searchLower := strings.ToLower(searchTerm)
	for _, s := range slice {
		if strings.Contains(strings.ToLower(s), searchLower) {
			return true
		}
	}
	return false
}