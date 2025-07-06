package swagger

import (
	"fmt"
	"regexp"
	"strings"

	"go.uber.org/zap"
	"swagger-docs-mcp/pkg/types"
	"swagger-docs-mcp/pkg/utils"
)

// ToolGenerator generates MCP tools from swagger documents
type ToolGenerator struct {
	logger *utils.Logger
}

// NewToolGenerator creates a new tool generator
func NewToolGenerator(logger *utils.Logger) *ToolGenerator {
	return &ToolGenerator{
		logger: logger.Child("generator"),
	}
}

// GenerateToolsFromDocument generates MCP tools from a parsed swagger document
func (g *ToolGenerator) GenerateToolsFromDocument(document *types.SwaggerDocument, docInfo *types.SwaggerDocumentInfo) ([]*types.GeneratedTool, error) {
	g.logger.Debug("Generating tools from document", zap.String("title", docInfo.Title))

	// Extract endpoints from the document
	parser := NewParser(g.logger)
	endpoints, err := parser.ExtractEndpoints(document)
	if err != nil {
		return nil, fmt.Errorf("failed to extract endpoints: %w", err)
	}

	var tools []*types.GeneratedTool
	for _, endpoint := range endpoints {
		// Skip deprecated endpoints if configured
		if endpoint.Deprecated {
			g.logger.Debug("Skipping deprecated endpoint", zap.String("method", endpoint.Method), zap.String("path", endpoint.Path))
			continue
		}

		tool, err := g.generateToolFromEndpoint(&endpoint, docInfo)
		if err != nil {
			g.logger.Error("Failed to generate tool for endpoint", zap.String("method", endpoint.Method), zap.String("path", endpoint.Path), zap.Error(err))
			continue
		}

		tools = append(tools, tool)
	}

	g.logger.Debug("Generated tools from document", zap.Int("toolCount", len(tools)), zap.String("title", docInfo.Title))
	return tools, nil
}

// generateToolFromEndpoint generates a single MCP tool from a swagger endpoint
func (g *ToolGenerator) generateToolFromEndpoint(endpoint *types.SwaggerEndpoint, docInfo *types.SwaggerDocumentInfo) (*types.GeneratedTool, error) {
	// Generate tool name
	toolName := g.generateToolName(endpoint, docInfo)

	// Generate tool description
	description := g.generateToolDescription(endpoint, docInfo)

	// Generate input schema
	inputSchema, err := g.generateInputSchema(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to generate input schema: %w", err)
	}

	tool := &types.GeneratedTool{
		Name:         toolName,
		Description:  description,
		InputSchema:  inputSchema,
		Endpoint:     endpoint,
		DocumentInfo: docInfo,
	}

	return tool, nil
}

// generateToolName generates a unique tool name for an endpoint
func (g *ToolGenerator) generateToolName(endpoint *types.SwaggerEndpoint, docInfo *types.SwaggerDocumentInfo) string {
	var baseName string

	// Use operation ID if available
	if endpoint.OperationID != "" {
		baseName = g.sanitizeToolName(endpoint.OperationID)
	} else {
		// Generate from path and method
		pathParts := strings.Split(strings.Trim(endpoint.Path, "/"), "/")
		var cleanParts []string

		for _, part := range pathParts {
			// Remove parameter placeholders
			if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
				paramName := strings.Trim(part, "{}")
				cleanParts = append(cleanParts, paramName)
			} else {
				cleanParts = append(cleanParts, part)
			}
		}

		pathStr := strings.Join(cleanParts, "_")
		method := strings.ToLower(endpoint.Method)
		baseName = g.sanitizeToolName(fmt.Sprintf("%s_%s", pathStr, method))
	}

	// Add document title suffix if the base name might conflict
	// This helps distinguish between similar endpoints from different documents
	if g.shouldAddDocumentSuffix(baseName, docInfo) {
		docSuffix := g.createDocumentSuffix(docInfo.Title)
		if docSuffix != "" {
			baseName = fmt.Sprintf("%s_%s", baseName, docSuffix)
		}
	}

	// Add version number at the end
	if docInfo.Version != "" {
		baseName = fmt.Sprintf("%s_v%s", baseName, docInfo.Version)
	}

	return baseName
}

// shouldAddDocumentSuffix determines if we should add a document suffix to avoid conflicts
func (g *ToolGenerator) shouldAddDocumentSuffix(baseName string, docInfo *types.SwaggerDocumentInfo) bool {
	// Add suffix for common operation IDs that might appear in multiple documents
	commonOperationIDs := []string{
		"wx_forecast_fifteenminute_get",
		"get_current",
		"get_forecast",
		"get_observations",
	}

	for _, commonID := range commonOperationIDs {
		if baseName == commonID {
			return true
		}
	}

	// Also check for any forecast endpoints that might conflict
	if strings.Contains(baseName, "forecast") && strings.Contains(baseName, "fifteenminute") {
		return true
	}

	return false
}

// createDocumentSuffix creates a short suffix from document title
func (g *ToolGenerator) createDocumentSuffix(title string) string {
	// Extract meaningful parts from title
	words := strings.Fields(strings.ToLower(title))
	var meaningfulWords []string

	// Skip common words
	skipWords := map[string]bool{
		"v1": true, "v2": true, "v3": true,
		"api": true, "service": true, "the": true,
		"and": true, "or": true, "of": true,
	}

	for _, word := range words {
		if !skipWords[word] && len(word) > 2 {
			meaningfulWords = append(meaningfulWords, word)
			if len(meaningfulWords) >= 2 { // Limit to 2 words max
				break
			}
		}
	}

	if len(meaningfulWords) == 0 {
		return ""
	}

	suffix := strings.Join(meaningfulWords, "_")
	return g.sanitizeToolName(suffix)
}

// generateToolDescription generates a description for the tool
func (g *ToolGenerator) generateToolDescription(endpoint *types.SwaggerEndpoint, docInfo *types.SwaggerDocumentInfo) string {
	// Start with endpoint summary or description
	description := endpoint.Summary
	if description == "" {
		description = endpoint.Description
	}

	// If no description available, generate one
	if description == "" {
		description = fmt.Sprintf("%s %s", endpoint.Method, endpoint.Path)
	}

	// Add API version info
	if docInfo.Version != "" {
		description = fmt.Sprintf("[v%s] %s", docInfo.Version, description)
	}

	// Add tags if available
	if len(endpoint.Tags) > 0 {
		description = fmt.Sprintf("%s (Tags: %s)", description, strings.Join(endpoint.Tags, ", "))
	}

	// Truncate if too long (default max 200 characters)
	maxLength := 200
	if len(description) > maxLength {
		description = description[:maxLength-3] + "..."
	}

	return description
}

// generateInputSchema generates JSON schema for tool input parameters
func (g *ToolGenerator) generateInputSchema(endpoint *types.SwaggerEndpoint) (map[string]interface{}, error) {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": make(map[string]interface{}),
		"required":   []string{},
	}

	properties := schema["properties"].(map[string]interface{})
	var required []string

	// Add parameters to schema
	for _, param := range endpoint.Parameters {
		paramSchema := g.generateParameterSchema(&param)
		properties[param.Name] = paramSchema

		if param.Required {
			required = append(required, param.Name)
		}
	}

	// Add request body if present
	if endpoint.RequestBody != nil {
		if requestBodyMap, ok := endpoint.RequestBody.(map[string]interface{}); ok {
			if content, ok := requestBodyMap["content"].(map[string]interface{}); ok {
				// Look for JSON content type
				for contentType, contentSchema := range content {
					if strings.Contains(contentType, "json") {
						if schemaMap, ok := contentSchema.(map[string]interface{}); ok {
							if schema, ok := schemaMap["schema"].(map[string]interface{}); ok {
								properties["requestBody"] = schema

								// Check if request body is required
								if requiredVal, ok := requestBodyMap["required"].(bool); ok && requiredVal {
									required = append(required, "requestBody")
								}
							}
						}
						break
					}
				}
			}
		}
	}

	schema["required"] = required
	return schema, nil
}

// generateParameterSchema generates schema for a single parameter
func (g *ToolGenerator) generateParameterSchema(param *types.SwaggerParameter) map[string]interface{} {
	schema := map[string]interface{}{
		"type": "string", // Default to string
	}

	if param.Description != "" {
		schema["description"] = param.Description
	}

	// Extract type from parameter schema
	if param.Schema != nil {
		if schemaMap, ok := param.Schema.(map[string]interface{}); ok {
			// Copy relevant schema properties
			if paramType, ok := schemaMap["type"].(string); ok {
				schema["type"] = paramType
			}
			if format, ok := schemaMap["format"].(string); ok {
				schema["format"] = format
			}
			if enum, ok := schemaMap["enum"].([]interface{}); ok {
				schema["enum"] = enum
			}
			if minimum, ok := schemaMap["minimum"]; ok {
				schema["minimum"] = minimum
			}
			if maximum, ok := schemaMap["maximum"]; ok {
				schema["maximum"] = maximum
			}
			if pattern, ok := schemaMap["pattern"].(string); ok {
				schema["pattern"] = pattern
			}
		}
	}

	// Add example if available
	if param.Example != nil {
		schema["example"] = param.Example
	}

	// Add parameter location as metadata
	schema["x-parameter-in"] = param.In

	return schema
}

// sanitizeToolName sanitizes a tool name to be valid
func (g *ToolGenerator) sanitizeToolName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace invalid characters with underscores
	reg := regexp.MustCompile(`[^a-z0-9_]`)
	name = reg.ReplaceAllString(name, "_")

	// Remove multiple consecutive underscores
	reg = regexp.MustCompile(`_+`)
	name = reg.ReplaceAllString(name, "_")

	// Remove leading/trailing underscores
	name = strings.Trim(name, "_")

	// Ensure name is not empty
	if name == "" {
		name = "unknown_tool"
	}

	return name
}

// GetToolStatistics returns statistics about tool generation
func (g *ToolGenerator) GetToolStatistics(tools []*types.GeneratedTool) map[string]interface{} {
	stats := map[string]interface{}{
		"totalTools": len(tools),
	}

	// Count by method
	methodCounts := make(map[string]int)
	for _, tool := range tools {
		if tool.Endpoint != nil {
			method := tool.Endpoint.Method
			methodCounts[method]++
		}
	}
	stats["toolsByMethod"] = methodCounts

	// Count by version
	versionCounts := make(map[string]int)
	for _, tool := range tools {
		if tool.DocumentInfo != nil {
			version := tool.DocumentInfo.Version
			versionCounts[version]++
		}
	}
	stats["toolsByVersion"] = versionCounts

	// Count by tags
	tagCounts := make(map[string]int)
	for _, tool := range tools {
		if tool.Endpoint != nil {
			for _, tag := range tool.Endpoint.Tags {
				tagCounts[tag]++
			}
		}
	}
	stats["toolsByTag"] = tagCounts

	return stats
}
