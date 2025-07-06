package swagger

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"swagger-docs-mcp/pkg/types"
	"swagger-docs-mcp/pkg/utils"
)

// Parser handles swagger document parsing and validation
type Parser struct {
	logger *utils.Logger
}

// NewParser creates a new swagger document parser
func NewParser(logger *utils.Logger) *Parser {
	return &Parser{
		logger: logger.Child("parser"),
	}
}

// ParseDocument parses a swagger document from file or URL
func (p *Parser) ParseDocument(filePath string) (*types.SwaggerDocument, error) {
	p.logger.Debug("Parsing document", zap.String("filePath", filePath))

	var content []byte
	var err error

	// Check if it's a URL or local file
	if isURL(filePath) {
		content, err = p.fetchURL(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch URL %s: %w", filePath, err)
		}
	} else {
		content, err = ioutil.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
		}
	}

	// Determine format from file extension or content
	format := p.detectFormat(filePath, content)

	// Parse the content
	document, err := p.parseContent(content, format)
	if err != nil {
		return nil, fmt.Errorf("failed to parse document %s (format: %s, size: %d bytes): %w", filePath, format, len(content), err)
	}

	p.logger.Debug("Successfully parsed document", zap.String("filePath", filePath))
	return document, nil
}

// ParseDocumentWithContent parses a swagger document from pre-fetched content
func (p *Parser) ParseDocumentWithContent(docInfo *types.SwaggerDocumentInfo) (*types.SwaggerDocument, error) {
	p.logger.Debug("Parsing document with content", zap.String("filePath", docInfo.FilePath))

	if len(docInfo.Content) == 0 {
		// Fall back to regular parsing if no content is stored
		return p.ParseDocument(docInfo.FilePath)
	}

	// Determine format from file path or content
	format := p.detectFormat(docInfo.FilePath, docInfo.Content)

	// Parse the content
	document, err := p.parseContent(docInfo.Content, format)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pre-fetched document %s (format: %s, content size: %d bytes): %w", docInfo.FilePath, format, len(docInfo.Content), err)
	}

	p.logger.Debug("Successfully parsed document with content", zap.String("filePath", docInfo.FilePath))
	return document, nil
}

// ParseContent parses swagger content from bytes
func (p *Parser) ParseContent(content []byte, format string) (*types.SwaggerDocument, error) {
	return p.parseContent(content, format)
}

// parseContent parses the content based on format
func (p *Parser) parseContent(content []byte, format string) (*types.SwaggerDocument, error) {
	var document types.SwaggerDocument

	switch strings.ToLower(format) {
	case "json":
		if err := json.Unmarshal(content, &document); err != nil {
			return nil, fmt.Errorf("JSON parsing error (content preview: %.100s...): %w", string(content), err)
		}
	case "yaml", "yml":
		if err := yaml.Unmarshal(content, &document); err != nil {
			return nil, fmt.Errorf("YAML parsing error (content preview: %.100s...): %w", string(content), err)
		}
	default:
		// Try JSON first, then YAML
		jsonErr := json.Unmarshal(content, &document)
		if jsonErr != nil {
			yamlErr := yaml.Unmarshal(content, &document)
			if yamlErr != nil {
				return nil, fmt.Errorf("failed to parse as JSON (error: %v) or YAML (error: %v) - content preview: %.100s...", jsonErr, yamlErr, string(content))
			}
		}
	}

	// Validate that it's a valid swagger/openapi document
	if err := p.validateDocument(&document); err != nil {
		return nil, fmt.Errorf("document validation failed - not a valid OpenAPI/Swagger document (openapi: %s, swagger: %s, info.title: %s): %w",
			document.OpenAPI, document.Swagger, getInfoTitle(&document), err)
	}

	return &document, nil
}

// ExtractEndpoints extracts endpoints from a swagger document
func (p *Parser) ExtractEndpoints(document *types.SwaggerDocument) ([]types.SwaggerEndpoint, error) {
	var endpoints []types.SwaggerEndpoint

	if document.Paths == nil {
		return endpoints, nil
	}

	for path, pathItemInterface := range document.Paths {
		pathItem, ok := pathItemInterface.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract endpoints for each HTTP method
		for method, operationInterface := range pathItem {
			// Skip non-HTTP methods
			if !isHTTPMethod(method) {
				p.logger.Debug("Skipping non-HTTP method", zap.String("method", method), zap.String("path", path))
				continue
			}

			operation, ok := operationInterface.(map[string]interface{})
			if !ok {
				p.logger.Debug("Skipping invalid operation - not a map", zap.String("method", method), zap.String("path", path))
				continue
			}

			endpoint := types.SwaggerEndpoint{
				Path:   path,
				Method: strings.ToUpper(method),
			}

			// Extract basic operation details
			if operationID, ok := operation["operationId"].(string); ok {
				endpoint.OperationID = operationID
			}

			if summary, ok := operation["summary"].(string); ok {
				endpoint.Summary = summary
			}

			if description, ok := operation["description"].(string); ok {
				endpoint.Description = description
			}

			if deprecated, ok := operation["deprecated"].(bool); ok {
				endpoint.Deprecated = deprecated
			}

			// Extract tags
			if tagsInterface, ok := operation["tags"].([]interface{}); ok {
				for _, tagInterface := range tagsInterface {
					if tag, ok := tagInterface.(string); ok {
						endpoint.Tags = append(endpoint.Tags, tag)
					}
				}
			}

			// Extract parameters
			if parametersInterface, ok := operation["parameters"].([]interface{}); ok {
				for _, paramInterface := range parametersInterface {
					if paramMap, ok := paramInterface.(map[string]interface{}); ok {
						param := p.parseParameter(paramMap)
						endpoint.Parameters = append(endpoint.Parameters, param)
					}
				}
			}

			// Extract global parameters from path level
			if globalParametersInterface, ok := pathItem["parameters"].([]interface{}); ok {
				for _, paramInterface := range globalParametersInterface {
					if paramMap, ok := paramInterface.(map[string]interface{}); ok {
						param := p.parseParameter(paramMap)
						endpoint.Parameters = append(endpoint.Parameters, param)
					}
				}
			}

			// Extract request body
			if requestBody, ok := operation["requestBody"]; ok {
				endpoint.RequestBody = requestBody
			}

			// Extract responses
			if responses, ok := operation["responses"].(map[string]interface{}); ok {
				endpoint.Responses = responses
			}

			// Extract security
			if security, ok := operation["security"].([]interface{}); ok {
				endpoint.Security = security
			}

			endpoints = append(endpoints, endpoint)
		}
	}

	p.logger.Debug("Extracted endpoints", zap.Int("count", len(endpoints)))
	return endpoints, nil
}

// parseParameter parses a parameter object
func (p *Parser) parseParameter(paramMap map[string]interface{}) types.SwaggerParameter {
	param := types.SwaggerParameter{}

	if name, ok := paramMap["name"].(string); ok {
		param.Name = name
	}

	if in, ok := paramMap["in"].(string); ok {
		param.In = in
	}

	if description, ok := paramMap["description"].(string); ok {
		param.Description = description
	}

	if required, ok := paramMap["required"].(bool); ok {
		param.Required = required
	}

	if schema, ok := paramMap["schema"]; ok {
		param.Schema = schema
	}

	if example, ok := paramMap["example"]; ok {
		param.Example = example
	}

	return param
}

// detectFormat detects the format of the content
func (p *Parser) detectFormat(filePath string, content []byte) string {
	// First try to detect from file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	}

	// Try to detect from content
	trimmed := strings.TrimSpace(string(content))
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return "json"
	}

	// Default to YAML for unknown formats
	return "yaml"
}

// fetchURL fetches content from a URL
func (p *Parser) fetchURL(urlStr string) ([]byte, error) {
	// This should use the same HTTP client as the scanner
	// For now, return an error as URL fetching is handled by the scanner
	return nil, fmt.Errorf("URL parsing not supported in parser - use scanner for URLs")
}

// validateDocument validates that the document is a valid swagger/openapi document
func (p *Parser) validateDocument(document *types.SwaggerDocument) error {
	// Check for OpenAPI or Swagger version
	if document.OpenAPI == "" && document.Swagger == "" {
		return fmt.Errorf("missing required version field - document must have either 'openapi' or 'swagger' field")
	}

	// Check for info section
	if document.Info == nil {
		return fmt.Errorf("missing required 'info' section - OpenAPI/Swagger documents must have an info object")
	}

	if document.Info.Title == "" {
		return fmt.Errorf("missing required 'info.title' field - API title is mandatory")
	}

	if document.Info.Version == "" {
		return fmt.Errorf("missing required 'info.version' field - API version is mandatory")
	}

	// Check for paths
	if document.Paths == nil {
		p.logger.Warn("Document has no paths defined - no API endpoints will be available for tool generation")
	} else if len(document.Paths) == 0 {
		p.logger.Warn("Document has empty paths object - no API endpoints will be available for tool generation")
	}

	return nil
}

// isURL checks if a string is a URL
func isURL(str string) bool {
	u, err := url.Parse(str)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}

// isHTTPMethod checks if a string is a valid HTTP method
func isHTTPMethod(method string) bool {
	httpMethods := []string{
		"get", "post", "put", "delete", "patch", "head", "options", "trace",
	}

	method = strings.ToLower(method)
	for _, validMethod := range httpMethods {
		if method == validMethod {
			return true
		}
	}

	return false
}

// GetDocumentInfo extracts basic document information
func (p *Parser) GetDocumentInfo(document *types.SwaggerDocument) types.SwaggerDocumentInfo {
	info := types.SwaggerDocumentInfo{}

	if document.Info != nil {
		info.Title = document.Info.Title
		info.Version = document.Info.Version
	}

	// Extract package IDs from extension fields
	info.PackageIDs = p.extractStringArray(document.XSolaraPackageIDs)
	info.TwcDomainPortfolio = p.extractStringArray(document.XTwcDomainPortfolio)
	info.TwcDomain = p.extractStringArray(document.XTwcDomain)
	info.TwcUsageClassification = p.extractStringArray(document.XTwcUsageClassification)
	info.TwcGeography = p.extractStringArray(document.XTwcGeography)

	return info
}

// extractStringArray converts interface{} to []string, handling both strings and arrays
func (p *Parser) extractStringArray(value interface{}) []string {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case []interface{}:
		var result []string
		for _, item := range v {
			if str, ok := item.(string); ok && str != "" {
				result = append(result, str)
			}
		}
		return result
	case []string:
		return v
	default:
		p.logger.Debug("Unexpected type for extension field", zap.String("type", fmt.Sprintf("%T", v)))
		return nil
	}
}

// getInfoTitle safely extracts the title from document info
func getInfoTitle(document *types.SwaggerDocument) string {
	if document.Info == nil {
		return "<no info>"
	}
	if document.Info.Title == "" {
		return "<no title>"
	}
	return document.Info.Title
}
