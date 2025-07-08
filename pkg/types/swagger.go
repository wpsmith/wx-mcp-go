package types

import "time"

// SwaggerDocument represents a swagger/OpenAPI document
type SwaggerDocument struct {
	OpenAPI      string                 `json:"openapi,omitempty" yaml:"openapi,omitempty"`
	Swagger      string                 `json:"swagger,omitempty" yaml:"swagger,omitempty"`
	Info         *SwaggerInfo           `json:"info,omitempty" yaml:"info,omitempty"`
	Servers      []SwaggerServer        `json:"servers,omitempty" yaml:"servers,omitempty"`
	Paths        map[string]interface{} `json:"paths,omitempty" yaml:"paths,omitempty"`
	Components   interface{}            `json:"components,omitempty" yaml:"components,omitempty"`
	Security     []interface{}          `json:"security,omitempty" yaml:"security,omitempty"`
	Tags         []interface{}          `json:"tags,omitempty" yaml:"tags,omitempty"`
	ExternalDocs interface{}            `json:"externalDocs,omitempty" yaml:"externalDocs,omitempty"`

	// Extension fields - use interface{} to handle both strings and arrays
	XSolaraPackageIDs       interface{} `json:"x-package-ids,omitempty" yaml:"x-package-ids,omitempty"`
	XTwcDomainPortfolio     interface{} `json:"x-twc-domain-portfolio,omitempty" yaml:"x-twc-domain-portfolio,omitempty"`
	XTwcDomain              interface{} `json:"x-twc-domain,omitempty" yaml:"x-twc-domain,omitempty"`
	XTwcUsageClassification interface{} `json:"x-twc-usage-classification,omitempty" yaml:"x-twc-usage-classification,omitempty"`
	XTwcGeography           interface{} `json:"x-twc-geography,omitempty" yaml:"x-twc-geography,omitempty"`
}

// SwaggerInfo represents swagger info section
type SwaggerInfo struct {
	Title          string      `json:"title" yaml:"title"`
	Description    string      `json:"description,omitempty" yaml:"description,omitempty"`
	Version        string      `json:"version" yaml:"version"`
	TermsOfService string      `json:"termsOfService,omitempty" yaml:"termsOfService,omitempty"`
	Contact        interface{} `json:"contact,omitempty" yaml:"contact,omitempty"`
	License        interface{} `json:"license,omitempty" yaml:"license,omitempty"`
}

// SwaggerServer represents a swagger server
type SwaggerServer struct {
	URL         string                 `json:"url" yaml:"url"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Variables   map[string]interface{} `json:"variables,omitempty" yaml:"variables,omitempty"`
}

// SwaggerEndpoint represents a swagger endpoint
type SwaggerEndpoint struct {
	Path        string                 `json:"path"`
	Method      string                 `json:"method"`
	OperationID string                 `json:"operationId,omitempty"`
	Summary     string                 `json:"summary,omitempty"`
	Description string                 `json:"description,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Parameters  []SwaggerParameter     `json:"parameters,omitempty"`
	RequestBody interface{}            `json:"requestBody,omitempty"`
	Responses   map[string]interface{} `json:"responses,omitempty"`
	Security    []interface{}          `json:"security,omitempty"`
	Deprecated  bool                   `json:"deprecated,omitempty"`
	MCPToolName string                 `json:"x-mcp-tool-name,omitempty"`
}

// SwaggerParameter represents a swagger parameter
type SwaggerParameter struct {
	Name        string      `json:"name"`
	In          string      `json:"in"`
	Description string      `json:"description,omitempty"`
	Required    bool        `json:"required,omitempty"`
	Schema      interface{} `json:"schema,omitempty"`
	Example     interface{} `json:"example,omitempty"`
}

// SwaggerDocumentInfo represents metadata about a swagger document
type SwaggerDocumentInfo struct {
	FilePath               string            `json:"filePath"`
	Version                string            `json:"version"`
	Title                  string            `json:"title"`
	Endpoints              []SwaggerEndpoint `json:"endpoints"`
	IsRemote               bool              `json:"isRemote,omitempty"`
	PackageIDs             []string          `json:"packageIds,omitempty"`
	TwcDomainPortfolio     []string          `json:"twcDomainPortfolio,omitempty"`
	TwcDomain              []string          `json:"twcDomain,omitempty"`
	TwcUsageClassification []string          `json:"twcUsageClassification,omitempty"`
	TwcGeography           []string          `json:"twcGeography,omitempty"`
	LastModified           *time.Time        `json:"lastModified,omitempty"`
	Content                []byte            `json:"-"` // Store fetched content for remote docs
}

// ScanOptions represents options for scanning swagger documents
type ScanOptions struct {
	IncludeSubdirectories bool     `json:"includeSubdirectories"`
	SupportedExtensions   []string `json:"supportedExtensions"`
	MaxDepth              int      `json:"maxDepth"`
}

// ScanResult represents the result of a swagger document scan
type ScanResult struct {
	Documents []SwaggerDocumentInfo `json:"documents"`
	Errors    []ScanError           `json:"errors"`
	Stats     ScanStats             `json:"stats"`
}

// ScanError represents an error that occurred during scanning
type ScanError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// ScanStats represents statistics from a scan operation
type ScanStats struct {
	TotalFiles     int           `json:"totalFiles"`
	ValidDocuments int           `json:"validDocuments"`
	Errors         int           `json:"errors"`
	ScanTime       time.Duration `json:"scanTime"`
}

// DefaultScanOptions returns default scan options
func DefaultScanOptions() *ScanOptions {
	return &ScanOptions{
		IncludeSubdirectories: true,
		SupportedExtensions:   []string{".json", ".yaml", ".yml"},
		MaxDepth:              3,
	}
}
