package swagger

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"swagger-docs-mcp/pkg/types"
	"swagger-docs-mcp/pkg/utils"
)

// ResourceGenerator generates resources from Swagger documents
type ResourceGenerator struct {
	logger *utils.Logger
	config *types.ResourcesConfig
}

// NewResourceGenerator creates a new resource generator
func NewResourceGenerator(logger *utils.Logger, config *types.ResourcesConfig) *ResourceGenerator {
	return &ResourceGenerator{
		logger: logger.Child("resource-generator"),
		config: config,
	}
}

// GenerateResourcesFromDocument generates resources from a parsed Swagger document
func (g *ResourceGenerator) GenerateResourcesFromDocument(doc *types.SwaggerDocument, docInfo *types.SwaggerDocumentInfo) ([]*types.GeneratedResource, error) {
	if !g.config.Enabled {
		return nil, nil
	}

	// Extract endpoints from the document
	parser := NewParser(g.logger)
	endpoints, err := parser.ExtractEndpoints(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to extract endpoints: %w", err)
	}

	var resources []*types.GeneratedResource
	
	// Generate documentation resources
	if g.config.ExposeSwaggerDocs {
		docResources := g.generateDocumentationResources(doc, endpoints, docInfo)
		resources = append(resources, docResources...)
	}

	// Generate schema resources
	schemaResources := g.generateSchemaResources(doc, docInfo)
	resources = append(resources, schemaResources...)

	// Generate example resources
	exampleResources := g.generateExampleResources(endpoints, docInfo)
	resources = append(resources, exampleResources...)

	// Generate endpoint discovery resources
	if g.config.AllowEndpointDiscovery {
		endpointResources := g.generateEndpointResources(endpoints, docInfo)
		resources = append(resources, endpointResources...)
	}

	g.logger.Debug("Generated resources from document",
		zap.String("document", docInfo.FilePath),
		zap.Int("resourceCount", len(resources)))

	return resources, nil
}

// generateDocumentationResources generates documentation resources
func (g *ResourceGenerator) generateDocumentationResources(doc *types.SwaggerDocument, endpoints []types.SwaggerEndpoint, docInfo *types.SwaggerDocumentInfo) []*types.GeneratedResource {
	var resources []*types.GeneratedResource

	// Full Swagger document resource
	swaggerResource := &types.GeneratedResource{
		URI:         g.createResourceURI(docInfo, "swagger", "json"),
		Name:        g.createResourceName(docInfo, "Swagger Document"),
		Description: fmt.Sprintf("Complete Swagger/OpenAPI specification for %s", docInfo.Title),
		MimeType:    "application/json",
		Category:    types.ResourceCategoryDocumentation,
		Tags:        []string{"swagger", "openapi", "specification"},
		Source:      docInfo,
		Metadata: map[string]interface{}{
			"version":   docInfo.Version,
			"title":     docInfo.Title,
			"endpoints": len(endpoints),
			"schemas":   0, // TODO: extract schemas from components or definitions
		},
	}
	resources = append(resources, swaggerResource)

	// API overview resource
	overviewResource := &types.GeneratedResource{
		URI:         g.createResourceURI(docInfo, "overview", "md"),
		Name:        g.createResourceName(docInfo, "API Overview"),
		Description: fmt.Sprintf("Human-readable overview of the %s API", docInfo.Title),
		MimeType:    "text/markdown",
		Category:    types.ResourceCategoryDocumentation,
		Tags:        []string{"overview", "documentation", "summary"},
		Source:      docInfo,
		Metadata: map[string]interface{}{
			"endpoints": len(endpoints),
			"categories": g.getEndpointCategories(endpoints),
		},
	}
	resources = append(resources, overviewResource)

	return resources
}

// generateSchemaResources generates schema resources
func (g *ResourceGenerator) generateSchemaResources(doc *types.SwaggerDocument, docInfo *types.SwaggerDocumentInfo) []*types.GeneratedResource {
	var resources []*types.GeneratedResource

	// TODO: Extract schemas from components or definitions
	// For now, return empty to avoid compilation errors
	schemas := make(map[string]interface{})

	// Generate individual schema resources
	for schemaName, schema := range schemas {
		schemaResource := &types.GeneratedResource{
			URI:         g.createResourceURI(docInfo, fmt.Sprintf("schema-%s", schemaName), "json"),
			Name:        fmt.Sprintf("%s Schema", schemaName),
			Description: fmt.Sprintf("JSON schema definition for %s", schemaName),
			MimeType:    "application/json",
			Category:    types.ResourceCategorySchema,
			Tags:        []string{"schema", "json-schema", schemaName},
			Source:      docInfo,
			Metadata: map[string]interface{}{
				"schemaName": schemaName,
				"type":       g.getSchemaType(schema),
			},
		}
		resources = append(resources, schemaResource)
	}

	// Generate combined schemas resource
	if len(schemas) > 0 {
		allSchemasResource := &types.GeneratedResource{
			URI:         g.createResourceURI(docInfo, "schemas", "json"),
			Name:        g.createResourceName(docInfo, "All Schemas"),
			Description: fmt.Sprintf("All JSON schema definitions for %s", docInfo.Title),
			MimeType:    "application/json",
			Category:    types.ResourceCategorySchema,
			Tags:        []string{"schemas", "json-schema", "all"},
			Source:      docInfo,
			Metadata: map[string]interface{}{
				"schemaCount": len(schemas),
				"schemas":     g.getSchemaNames(schemas),
			},
		}
		resources = append(resources, allSchemasResource)
	}

	return resources
}

// generateExampleResources generates example resources
func (g *ResourceGenerator) generateExampleResources(endpoints []types.SwaggerEndpoint, docInfo *types.SwaggerDocumentInfo) []*types.GeneratedResource {
	var resources []*types.GeneratedResource

	// Generate examples for each endpoint
	for _, endpoint := range endpoints {
		if !g.hasExamples(&endpoint) {
			continue
		}

		exampleResource := &types.GeneratedResource{
			URI:         g.createEndpointResourceURI(docInfo, &endpoint, "example", "json"),
			Name:        fmt.Sprintf("%s %s Example", strings.ToUpper(endpoint.Method), endpoint.Path),
			Description: fmt.Sprintf("Example request and response for %s %s", endpoint.Method, endpoint.Path),
			MimeType:    "application/json",
			Category:    types.ResourceCategoryExample,
			Tags:        []string{"example", "request", "response", endpoint.Method},
			Source:      docInfo,
			Metadata: map[string]interface{}{
				"method":   endpoint.Method,
				"path":     endpoint.Path,
				"summary":  endpoint.Summary,
				"hasAuth":  len(endpoint.Security) > 0,
			},
		}
		resources = append(resources, exampleResource)
	}

	return resources
}

// generateEndpointResources generates endpoint discovery resources
func (g *ResourceGenerator) generateEndpointResources(endpoints []types.SwaggerEndpoint, docInfo *types.SwaggerDocumentInfo) []*types.GeneratedResource {
	var resources []*types.GeneratedResource

	// Endpoints catalog resource
	catalogResource := &types.GeneratedResource{
		URI:         g.createResourceURI(docInfo, "endpoints", "json"),
		Name:        g.createResourceName(docInfo, "Endpoints Catalog"),
		Description: fmt.Sprintf("Complete catalog of all endpoints in %s", docInfo.Title),
		MimeType:    "application/json",
		Category:    types.ResourceCategoryEndpoint,
		Tags:        []string{"endpoints", "catalog", "discovery"},
		Source:      docInfo,
		Metadata: map[string]interface{}{
			"endpointCount": len(endpoints),
			"methods":       g.getUniqueMethods(endpoints),
			"categories":    g.getEndpointCategories(endpoints),
		},
	}
	resources = append(resources, catalogResource)

	// Category-based endpoint resources
	categories := g.categorizeEndpoints(endpoints)
	for category, endpoints := range categories {
		if len(endpoints) == 0 {
			continue
		}

		categoryResource := &types.GeneratedResource{
			URI:         g.createResourceURI(docInfo, fmt.Sprintf("endpoints-%s", category), "json"),
			Name:        fmt.Sprintf("%s Endpoints", strings.Title(category)),
			Description: fmt.Sprintf("Endpoints related to %s functionality", category),
			MimeType:    "application/json",
			Category:    types.ResourceCategoryEndpoint,
			Tags:        []string{"endpoints", category, "filtered"},
			Source:      docInfo,
			Metadata: map[string]interface{}{
				"category":      category,
				"endpointCount": len(endpoints),
			},
		}
		resources = append(resources, categoryResource)
	}

	return resources
}

// Helper methods

// createResourceURI creates a URI for a resource
func (g *ResourceGenerator) createResourceURI(docInfo *types.SwaggerDocumentInfo, resourceType, format string) string {
	base := filepath.Base(docInfo.FilePath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	
	return fmt.Sprintf("swagger://%s/%s.%s", name, resourceType, format)
}

// createEndpointResourceURI creates a URI for an endpoint-specific resource
func (g *ResourceGenerator) createEndpointResourceURI(docInfo *types.SwaggerDocumentInfo, endpoint *types.SwaggerEndpoint, resourceType, format string) string {
	base := filepath.Base(docInfo.FilePath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	
	// Create safe endpoint identifier
	endpointID := g.createEndpointIdentifier(endpoint)
	
	return fmt.Sprintf("swagger://%s/endpoints/%s/%s.%s", name, endpointID, resourceType, format)
}

// createResourceName creates a display name for a resource
func (g *ResourceGenerator) createResourceName(docInfo *types.SwaggerDocumentInfo, suffix string) string {
	if docInfo.Title != "" {
		return fmt.Sprintf("%s %s", docInfo.Title, suffix)
	}
	
	base := filepath.Base(docInfo.FilePath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	return fmt.Sprintf("%s %s", strings.Title(name), suffix)
}

// createEndpointIdentifier creates a safe identifier for an endpoint
func (g *ResourceGenerator) createEndpointIdentifier(endpoint *types.SwaggerEndpoint) string {
	// Create identifier from method and path
	path := strings.ReplaceAll(endpoint.Path, "/", "-")
	path = strings.ReplaceAll(path, "{", "")
	path = strings.ReplaceAll(path, "}", "")
	path = strings.Trim(path, "-")
	
	return fmt.Sprintf("%s-%s", strings.ToLower(endpoint.Method), path)
}

// getSchemaType extracts the type from a schema
func (g *ResourceGenerator) getSchemaType(schema interface{}) string {
	if schemaMap, ok := schema.(map[string]interface{}); ok {
		if schemaType, exists := schemaMap["type"]; exists {
			if typeStr, ok := schemaType.(string); ok {
				return typeStr
			}
		}
	}
	return "unknown"
}

// getSchemaNames extracts schema names from schemas map
func (g *ResourceGenerator) getSchemaNames(schemas map[string]interface{}) []string {
	var names []string
	for name := range schemas {
		names = append(names, name)
	}
	return names
}

// hasExamples checks if an endpoint has examples
func (g *ResourceGenerator) hasExamples(endpoint *types.SwaggerEndpoint) bool {
	// Check if endpoint has parameter examples
	for _, param := range endpoint.Parameters {
		if param.Example != nil {
			return true
		}
	}
	
	// Check responses for examples - responses are map[string]interface{}
	// so we can't directly access Example field
	// For now, assume some endpoints have examples if they have responses
	return len(endpoint.Responses) > 0
}

// getUniqueMethods gets unique HTTP methods from endpoints
func (g *ResourceGenerator) getUniqueMethods(endpoints []types.SwaggerEndpoint) []string {
	methodSet := make(map[string]bool)
	for _, endpoint := range endpoints {
		methodSet[strings.ToUpper(endpoint.Method)] = true
	}
	
	var methods []string
	for method := range methodSet {
		methods = append(methods, method)
	}
	
	return methods
}

// getEndpointCategories gets categories from endpoints
func (g *ResourceGenerator) getEndpointCategories(endpoints []types.SwaggerEndpoint) []string {
	categories := g.categorizeEndpoints(endpoints)
	var categoryList []string
	for category := range categories {
		categoryList = append(categoryList, category)
	}
	return categoryList
}

// categorizeEndpoints categorizes endpoints by their functionality
func (g *ResourceGenerator) categorizeEndpoints(endpoints []types.SwaggerEndpoint) map[string][]*types.SwaggerEndpoint {
	categories := make(map[string][]*types.SwaggerEndpoint)
	
	for _, endpoint := range endpoints {
		category := g.categorizeEndpoint(&endpoint)
		if category == "" {
			category = "general"
		}
		categories[category] = append(categories[category], &endpoint)
	}
	
	return categories
}

// categorizeEndpoint categorizes a single endpoint
func (g *ResourceGenerator) categorizeEndpoint(endpoint *types.SwaggerEndpoint) string {
	path := strings.ToLower(endpoint.Path)
	summary := strings.ToLower(endpoint.Summary)
	description := strings.ToLower(endpoint.Description)
	
	text := fmt.Sprintf("%s %s %s", path, summary, description)
	
	// Weather-specific categories
	if g.containsAny(text, []string{"current", "conditions", "now", "present"}) {
		return "current"
	}
	if g.containsAny(text, []string{"forecast", "prediction", "future", "daily", "hourly"}) {
		return "forecast"
	}
	if g.containsAny(text, []string{"alert", "warning", "watch", "advisory"}) {
		return "alerts"
	}
	if g.containsAny(text, []string{"history", "historical", "past", "archive"}) {
		return "historical"
	}
	if g.containsAny(text, []string{"marine", "ocean", "sea", "wave", "tide"}) {
		return "marine"
	}
	if g.containsAny(text, []string{"aviation", "flight", "airport", "metar", "taf"}) {
		return "aviation"
	}
	if g.containsAny(text, []string{"lifestyle", "index", "comfort", "activity"}) {
		return "lifestyle"
	}
	
	return ""
}

// containsAny checks if text contains any of the given keywords
func (g *ResourceGenerator) containsAny(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

// GetResourceContent generates the actual content for a resource
func (g *ResourceGenerator) GetResourceContent(resource *types.GeneratedResource, doc *types.SwaggerDocument) (string, error) {
	uri, err := url.Parse(resource.URI)
	if err != nil {
		return "", fmt.Errorf("invalid resource URI: %w", err)
	}

	pathParts := strings.Split(strings.Trim(uri.Path, "/"), "/")
	if len(pathParts) < 1 {
		return "", fmt.Errorf("invalid resource path")
	}

	resourceType := pathParts[0]
	
	switch {
	case resourceType == "swagger.json":
		return g.generateSwaggerContent(doc)
	case resourceType == "overview.md":
		return g.generateOverviewContent(doc, resource.Source)
	case strings.HasPrefix(resourceType, "schema-"):
		schemaName := strings.TrimPrefix(resourceType, "schema-")
		schemaName = strings.TrimSuffix(schemaName, ".json")
		return g.generateSchemaContent(doc, schemaName)
	case resourceType == "schemas.json":
		return g.generateAllSchemasContent(doc)
	case resourceType == "endpoints.json":
		return g.generateEndpointsContent(doc)
	case strings.HasPrefix(resourceType, "endpoints-"):
		category := strings.TrimPrefix(resourceType, "endpoints-")
		category = strings.TrimSuffix(category, ".json")
		return g.generateCategoryEndpointsContent(doc, category)
	case strings.HasPrefix(resourceType, "endpoints/"):
		// Handle endpoint-specific resources
		return g.generateEndpointSpecificContent(doc, pathParts)
	default:
		return "", fmt.Errorf("unknown resource type: %s", resourceType)
	}
}

// generateSwaggerContent generates the full Swagger document content
func (g *ResourceGenerator) generateSwaggerContent(doc *types.SwaggerDocument) (string, error) {
	content, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal swagger document: %w", err)
	}
	return string(content), nil
}

// generateOverviewContent generates markdown overview content
func (g *ResourceGenerator) generateOverviewContent(doc *types.SwaggerDocument, docInfo *types.SwaggerDocumentInfo) (string, error) {
	var content strings.Builder
	
	content.WriteString(fmt.Sprintf("# %s API Overview\n\n", docInfo.Title))
	
	// Get description from doc.Info if available
	if doc.Info != nil && doc.Info.Description != "" {
		content.WriteString(fmt.Sprintf("%s\n\n", doc.Info.Description))
	}
	
	content.WriteString(fmt.Sprintf("**Version:** %s\n", docInfo.Version))
	// TODO: Extract base URL from servers if available
	content.WriteString("**Base URL:** N/A\n\n")
	
	content.WriteString("## Endpoints\n\n")
	
	// Extract endpoints first
	parser := NewParser(g.logger)
	endpoints, err := parser.ExtractEndpoints(doc)
	if err != nil {
		return "", fmt.Errorf("failed to extract endpoints: %w", err)
	}
	
	// Group endpoints by category
	categories := g.categorizeEndpoints(endpoints)
	for category, endpointList := range categories {
		content.WriteString(fmt.Sprintf("### %s\n\n", strings.Title(category)))
		
		for _, endpoint := range endpointList {
			content.WriteString(fmt.Sprintf("- **%s** `%s` - %s\n", 
				strings.ToUpper(endpoint.Method), endpoint.Path, endpoint.Summary))
		}
		content.WriteString("\n")
	}
	
	// TODO: Extract schemas and add data models section
	content.WriteString("## Data Models\n\n")
	content.WriteString("(Schema extraction not yet implemented)\n\n")
	
	return content.String(), nil
}

// generateSchemaContent generates content for a specific schema
func (g *ResourceGenerator) generateSchemaContent(doc *types.SwaggerDocument, schemaName string) (string, error) {
	// TODO: Extract schemas from components or definitions
	return "", fmt.Errorf("schema extraction not yet implemented")
}

// generateAllSchemasContent generates content for all schemas
func (g *ResourceGenerator) generateAllSchemasContent(doc *types.SwaggerDocument) (string, error) {
	// TODO: Extract schemas from components or definitions
	return "{}", nil
}

// generateEndpointsContent generates content for all endpoints
func (g *ResourceGenerator) generateEndpointsContent(doc *types.SwaggerDocument) (string, error) {
	// Extract endpoints first
	parser := NewParser(g.logger)
	endpoints, err := parser.ExtractEndpoints(doc)
	if err != nil {
		return "", fmt.Errorf("failed to extract endpoints: %w", err)
	}
	
	endpointList := make([]map[string]interface{}, 0, len(endpoints))
	
	for _, endpoint := range endpoints {
		endpointData := map[string]interface{}{
			"method":      endpoint.Method,
			"path":        endpoint.Path,
			"summary":     endpoint.Summary,
			"description": endpoint.Description,
			"parameters":  len(endpoint.Parameters),
			"responses":   len(endpoint.Responses),
			"security":    len(endpoint.Security) > 0,
		}
		endpointList = append(endpointList, endpointData)
	}
	
	content, err := json.MarshalIndent(endpointList, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal endpoints: %w", err)
	}
	
	return string(content), nil
}

// generateCategoryEndpointsContent generates content for category-specific endpoints
func (g *ResourceGenerator) generateCategoryEndpointsContent(doc *types.SwaggerDocument, category string) (string, error) {
	// Extract endpoints first
	parser := NewParser(g.logger)
	endpoints, err := parser.ExtractEndpoints(doc)
	if err != nil {
		return "", fmt.Errorf("failed to extract endpoints: %w", err)
	}
	
	categories := g.categorizeEndpoints(endpoints)
	categoryEndpoints, exists := categories[category]
	if !exists {
		return "", fmt.Errorf("category not found: %s", category)
	}
	
	endpointList := make([]map[string]interface{}, 0, len(categoryEndpoints))
	
	for _, endpoint := range categoryEndpoints {
		endpointData := map[string]interface{}{
			"method":      endpoint.Method,
			"path":        endpoint.Path,
			"summary":     endpoint.Summary,
			"description": endpoint.Description,
			"parameters":  len(endpoint.Parameters),
			"responses":   len(endpoint.Responses),
			"security":    len(endpoint.Security) > 0,
		}
		endpointList = append(endpointList, endpointData)
	}
	
	content, err := json.MarshalIndent(endpointList, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal category endpoints: %w", err)
	}
	
	return string(content), nil
}

// generateEndpointSpecificContent generates content for endpoint-specific resources
func (g *ResourceGenerator) generateEndpointSpecificContent(doc *types.SwaggerDocument, pathParts []string) (string, error) {
	// This would handle endpoint-specific resources like examples
	// Implementation depends on the specific structure needed
	return "{}", nil
}