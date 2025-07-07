package types

import "time"

// CLIOptions represents command-line interface options
type CLIOptions struct {
	Config       string   `mapstructure:"config"`
	SwaggerPaths []string `mapstructure:"swagger_paths"`
	SwaggerPath  []string `mapstructure:"swagger_path"`
	SwaggerURL   []string `mapstructure:"swagger_url"`
	PackageID    []string `mapstructure:"package_id"`
	Portfolio    []string `mapstructure:"portfolio"`
	Domain       []string `mapstructure:"domain"`
	Usage        []string `mapstructure:"usage"`
	Geography    []string `mapstructure:"geography"`
	Filter       []string `mapstructure:"filter"`
	Debug        bool     `mapstructure:"debug"`
	Verbose      bool     `mapstructure:"verbose"`
	Timeout      int      `mapstructure:"timeout"`
	MaxTools     int      `mapstructure:"max_tools"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Port     int           `mapstructure:"port" yaml:"port" json:"port"`
	Timeout  time.Duration `mapstructure:"timeout" yaml:"timeout" json:"timeout"`
	MaxTools int           `mapstructure:"max_tools" yaml:"maxTools" json:"maxTools"`
}

// HTTPConfig represents HTTP client configuration
type HTTPConfig struct {
	Timeout   time.Duration `mapstructure:"timeout" yaml:"timeout" json:"timeout"`
	Retries   int           `mapstructure:"retries" yaml:"retries" json:"retries"`
	UserAgent string        `mapstructure:"user_agent" yaml:"userAgent" json:"userAgent"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	APIKey        string            `mapstructure:"api_key" yaml:"apiKey" json:"apiKey"`
	DefaultScheme string            `mapstructure:"default_scheme" yaml:"defaultScheme" json:"defaultScheme"`
	Credentials   map[string]string `mapstructure:"credentials" yaml:"credentials" json:"credentials"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level   string `mapstructure:"level" yaml:"level" json:"level"`
	Enabled bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
}

// ToolGenerationConfig represents tool generation configuration
type ToolGenerationConfig struct {
	IncludeDeprecated    bool   `mapstructure:"include_deprecated" yaml:"includeDeprecated" json:"includeDeprecated"`
	MaxDescriptionLength int    `mapstructure:"max_description_length" yaml:"maxDescriptionLength" json:"maxDescriptionLength"`
	UseOperationID       bool   `mapstructure:"use_operation_id" yaml:"useOperationId" json:"useOperationId"`
	TagPrefix            string `mapstructure:"tag_prefix" yaml:"tagPrefix" json:"tagPrefix"`
}

// SwaggerProcessingConfig represents swagger processing configuration
type SwaggerProcessingConfig struct {
	ValidateDocuments bool `mapstructure:"validate_documents" yaml:"validateDocuments" json:"validateDocuments"`
	ResolveReferences bool `mapstructure:"resolve_references" yaml:"resolveReferences" json:"resolveReferences"`
	IgnoreErrors      bool `mapstructure:"ignore_errors" yaml:"ignoreErrors" json:"ignoreErrors"`
}

// TWCFilters represents TWC-specific filtering options
type TWCFilters struct {
	Portfolios           []string `mapstructure:"portfolios" yaml:"portfolios" json:"portfolios"`
	Domains              []string `mapstructure:"domains" yaml:"domains" json:"domains"`
	UsageClassifications []string `mapstructure:"usage_classifications" yaml:"usageClassifications" json:"usageClassifications"`
	Geographies          []string `mapstructure:"geographies" yaml:"geographies" json:"geographies"`
}

// PromptsConfig represents prompts configuration
type PromptsConfig struct {
	Enabled               bool     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	IncludeExamples       bool     `mapstructure:"include_examples" yaml:"includeExamples" json:"includeExamples"`
	GenerateFromEndpoints bool     `mapstructure:"generate_from_endpoints" yaml:"generateFromEndpoints" json:"generateFromEndpoints"`
	Categories            []string `mapstructure:"categories" yaml:"categories" json:"categories"`
}

// ResourcesConfig represents resources configuration
type ResourcesConfig struct {
	Enabled                   bool `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	ExposeSwaggerDocs         bool `mapstructure:"expose_swagger_docs" yaml:"exposeSwaggerDocs" json:"exposeSwaggerDocs"`
	EnableDocumentationSearch bool `mapstructure:"enable_documentation_search" yaml:"enableDocumentationSearch" json:"enableDocumentationSearch"`
	AllowEndpointDiscovery    bool `mapstructure:"allow_endpoint_discovery" yaml:"allowEndpointDiscovery" json:"allowEndpointDiscovery"`
}

// ConfigFile represents the configuration file format
type ConfigFile struct {
	Name              string                   `mapstructure:"name" yaml:"name" json:"name"`
	Version           string                   `mapstructure:"version" yaml:"version" json:"version"`
	SwaggerPaths      []string                 `mapstructure:"swagger_paths" yaml:"swaggerPaths" json:"swaggerPaths"`
	SwaggerURLs       []string                 `mapstructure:"swagger_urls" yaml:"swaggerUrls" json:"swaggerUrls"`
	PackageIDs        []string                 `mapstructure:"package_ids" yaml:"packageIds" json:"packageIds"`
	TWCFilters        *TWCFilters              `mapstructure:"twc_filters" yaml:"twcFilters" json:"twcFilters"`
	DynamicFilters    map[string]interface{}   `mapstructure:"dynamic_filters" yaml:"dynamicFilters" json:"dynamicFilters"`
	Server            *ServerConfig            `mapstructure:"server" yaml:"server" json:"server"`
	HTTP              *HTTPConfig              `mapstructure:"http" yaml:"http" json:"http"`
	Auth              *AuthConfig              `mapstructure:"auth" yaml:"auth" json:"auth"`
	Debug             bool                     `mapstructure:"debug" yaml:"debug" json:"debug"`
	Logging           *LoggingConfig           `mapstructure:"logging" yaml:"logging" json:"logging"`
	ToolGeneration    *ToolGenerationConfig    `mapstructure:"tool_generation" yaml:"toolGeneration" json:"toolGeneration"`
	SwaggerProcessing *SwaggerProcessingConfig `mapstructure:"swagger_processing" yaml:"swaggerProcessing" json:"swaggerProcessing"`
	Prompts           *PromptsConfig           `mapstructure:"prompts" yaml:"prompts" json:"prompts"`
	Resources         *ResourcesConfig         `mapstructure:"resources" yaml:"resources" json:"resources"`
}

// ResolvedConfig represents the final merged configuration
type ResolvedConfig struct {
	Name              string                  `json:"name"`
	Version           string                  `json:"version"`
	SwaggerPaths      []string                `json:"swaggerPaths"`
	SwaggerURLs       []string                `json:"swaggerUrls,omitempty"`
	PackageIDs        []string                `json:"packageIds,omitempty"`
	TWCFilters        *TWCFilters             `json:"twcFilters,omitempty"`
	DynamicFilters    map[string]interface{}  `json:"dynamicFilters,omitempty"`
	Server            ServerConfig            `json:"server"`
	HTTP              HTTPConfig              `json:"http"`
	Auth              AuthConfig              `json:"auth"`
	Debug             bool                    `json:"debug"`
	Logging           LoggingConfig           `json:"logging"`
	ToolGeneration    ToolGenerationConfig    `json:"toolGeneration"`
	SwaggerProcessing SwaggerProcessingConfig `json:"swaggerProcessing"`
	Prompts           PromptsConfig           `json:"prompts"`
	Resources         ResourcesConfig         `json:"resources"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *ResolvedConfig {
	return &ResolvedConfig{
		Name:         "swagger-docs-mcp",
		Version:      "1.0.0",
		SwaggerPaths: []string{},
		Server: ServerConfig{
			Port:     8080,
			Timeout:  30 * time.Second,
			MaxTools: 1000,
		},
		HTTP: HTTPConfig{
			Timeout:   10 * time.Second,
			Retries:   3,
			UserAgent: "swagger-docs-mcp/1.0.0",
		},
		Auth:  AuthConfig{},
		Debug: false,
		Logging: LoggingConfig{
			Level:   "info",
			Enabled: true,
		},
		ToolGeneration: ToolGenerationConfig{
			IncludeDeprecated:    false,
			MaxDescriptionLength: 500,
			UseOperationID:       true,
		},
		SwaggerProcessing: SwaggerProcessingConfig{
			ValidateDocuments: false,
			ResolveReferences: false,
			IgnoreErrors:      true,
		},
		Prompts: PromptsConfig{
			Enabled:               true,
			IncludeExamples:       true,
			GenerateFromEndpoints: true,
			Categories: []string{
				"current-conditions",
				"forecast",
				"alerts",
				"historical",
				"marine",
				"aviation",
				"lifestyle",
				"analysis",
				"comparison",
			},
		},
		Resources: ResourcesConfig{
			Enabled:                   true,
			ExposeSwaggerDocs:         true,
			EnableDocumentationSearch: true,
			AllowEndpointDiscovery:    true,
		},
	}
}
