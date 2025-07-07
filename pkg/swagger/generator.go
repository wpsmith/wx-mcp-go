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

// generateToolName generates a unique tool name for an endpoint (max 64 chars for MCP)
func (g *ToolGenerator) generateToolName(endpoint *types.SwaggerEndpoint, docInfo *types.SwaggerDocumentInfo) string {
	const maxToolNameLength = 64
	
	var baseName string

	// First check for x-mcp-tool-name and validate length
	if endpoint.MCPToolName != "" {
		toolName := strings.TrimSpace(endpoint.MCPToolName)
		if len(toolName) <= maxToolNameLength {
			return toolName
		}
		// If too long, log warning and fall back to generation
		g.logger.Warn("x-mcp-tool-name exceeds 64 characters, falling back to generated name", 
			zap.String("toolName", toolName), 
			zap.Int("length", len(toolName)))
	}

	// Use operation ID if available and not too long
	if endpoint.OperationID != "" {
		baseName = g.sanitizeToolName(endpoint.OperationID)
	} else {
		// Generate from path and method with length constraints
		baseName = g.generateCompactPathName(endpoint)
	}

	// Add version suffix efficiently
	versionSuffix := ""
	if docInfo.Version != "" {
		versionSuffix = fmt.Sprintf("_v%s", docInfo.Version)
	}

	// Calculate available space for base name
	availableLength := maxToolNameLength - len(versionSuffix)
	
	// Truncate base name if needed to fit within limit
	if len(baseName) > availableLength {
		// Try to preserve meaningful parts by abbreviating
		baseName = g.abbreviateToolName(baseName, availableLength)
	}

	finalName := baseName + versionSuffix
	
	// Final safety check
	if len(finalName) > maxToolNameLength {
		finalName = finalName[:maxToolNameLength-3] + "..." // Emergency truncation
		finalName = strings.TrimSuffix(finalName, "_") // Clean up trailing underscore
	}

	return finalName
}

// generateCompactPathName generates a compact name from endpoint path and method
func (g *ToolGenerator) generateCompactPathName(endpoint *types.SwaggerEndpoint) string {
	pathParts := strings.Split(strings.Trim(endpoint.Path, "/"), "/")
	var cleanParts []string

	for _, part := range pathParts {
		// Handle parameter placeholders
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			paramName := strings.Trim(part, "{}")
			// Abbreviate common parameter names
			switch paramName {
			case "locationId":
				cleanParts = append(cleanParts, "loc")
			case "latitude":
				cleanParts = append(cleanParts, "lat")
			case "longitude":
				cleanParts = append(cleanParts, "lon")
			case "geocode":
				cleanParts = append(cleanParts, "geo")
			default:
				if len(paramName) > 6 {
					cleanParts = append(cleanParts, paramName[:6])
				} else {
					cleanParts = append(cleanParts, paramName)
				}
			}
		} else {
			// Abbreviate common path parts
			abbreviated := g.abbreviatePathPart(part)
			if abbreviated != "" {
				cleanParts = append(cleanParts, abbreviated)
			}
		}
	}

	pathStr := strings.Join(cleanParts, "_")
	method := strings.ToLower(endpoint.Method)
	return g.sanitizeToolName(fmt.Sprintf("%s_%s", pathStr, method))
}

// abbreviatePathPart abbreviates common path parts to save space
func (g *ToolGenerator) abbreviatePathPart(part string) string {
	abbreviations := map[string]string{
		"forecast":     "fcst",
		"observations": "obs",
		"current":      "cur",
		"historical":   "hist",
		"location":     "loc",
		"geocode":      "geo",
		"notifications": "notif",
		"intraday":     "intra",
		"hourly":       "hr",
		"daily":        "day",
		"lightning":    "light",
		"temperature":  "temp",
		"humidity":     "humid",
		"pressure":     "press",
		"precipitation": "precip",
		"weather":      "wx",
		"almanac":      "alm",
		"astronomy":    "astro",
		"airquality":   "aq",
		"pollen":       "pol",
		"tides":        "tide",
	}

	if abbrev, exists := abbreviations[strings.ToLower(part)]; exists {
		return abbrev
	}

	// For other parts, truncate if too long
	if len(part) > 8 {
		return part[:8]
	}
	return part
}

// abbreviateToolName intelligently abbreviates a tool name to fit within the length limit
func (g *ToolGenerator) abbreviateToolName(name string, maxLength int) string {
	if len(name) <= maxLength {
		return name
	}

	// Split by underscores and abbreviate parts
	parts := strings.Split(name, "_")
	var abbreviatedParts []string
	
	for _, part := range parts {
		// Try to abbreviate this part
		abbreviated := g.abbreviatePathPart(part)
		abbreviatedParts = append(abbreviatedParts, abbreviated)
	}
	
	abbreviated := strings.Join(abbreviatedParts, "_")
	
	// If still too long, truncate from the end but preserve important parts
	if len(abbreviated) > maxLength {
		// Keep first few parts and method (usually last part)
		if len(abbreviatedParts) > 2 {
			firstParts := abbreviatedParts[:len(abbreviatedParts)-1]
			lastPart := abbreviatedParts[len(abbreviatedParts)-1]
			
			// Calculate space for first parts
			spaceForFirst := maxLength - len(lastPart) - 1 // -1 for underscore
			
			firstPartsStr := strings.Join(firstParts, "_")
			if len(firstPartsStr) > spaceForFirst {
				firstPartsStr = firstPartsStr[:spaceForFirst]
				firstPartsStr = strings.TrimSuffix(firstPartsStr, "_")
			}
			
			abbreviated = firstPartsStr + "_" + lastPart
		} else {
			// Just truncate
			abbreviated = abbreviated[:maxLength]
			abbreviated = strings.TrimSuffix(abbreviated, "_")
		}
	}
	
	return abbreviated
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
