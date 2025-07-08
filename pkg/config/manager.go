package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"swagger-docs-mcp/pkg/types"
)

// Manager handles configuration loading and validation
type Manager struct {
	configFileNames []string
}

// NewManager creates a new configuration manager
func NewManager() *Manager {
	return &Manager{
		configFileNames: []string{
			"swagger-mcp.config.json",
			"swagger-mcp.config.yaml",
			"swagger-mcp.config.yml",
			".swagger-mcp.json",
			".swagger-mcp.yaml",
			".swagger-mcp.yml",
		},
	}
}

// Load loads and merges configuration from multiple sources
func (m *Manager) Load(overrides *types.ResolvedConfig) (*types.ResolvedConfig, error) {
	// Start with default configuration
	config := types.DefaultConfig()

	// Load from configuration file
	fileConfig, err := m.loadConfigFile("")
	if err != nil {
		return nil, fmt.Errorf("failed to load config file: %w", err)
	}
	if fileConfig != nil {
		config = m.mergeConfig(config, fileConfig)
	}

	// Load from environment variables
	envConfig := m.loadEnvironmentConfig()
	config = m.mergeOverrides(config, envConfig)

	// Apply overrides
	if overrides != nil {
		config = m.mergeOverrides(config, overrides)
	}

	// Validate the final configuration
	if err := m.validateConfig(config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// LoadFromFile loads configuration from a specific file
func (m *Manager) LoadFromFile(configPath string, overrides *types.ResolvedConfig) (*types.ResolvedConfig, error) {
	config := types.DefaultConfig()

	fileConfig, err := m.loadConfigFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config file: %w", err)
	}
	if fileConfig != nil {
		config = m.mergeConfig(config, fileConfig)
	}

	envConfig := m.loadEnvironmentConfig()
	config = m.mergeOverrides(config, envConfig)

	if overrides != nil {
		config = m.mergeOverrides(config, overrides)
	}

	if err := m.validateConfig(config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// loadConfigFile loads configuration from file
func (m *Manager) loadConfigFile(configPath string) (*types.ConfigFile, error) {
	var filePath string

	if configPath != "" {
		// Use specified config file
		filePath = configPath
	} else {
		// Search for config file in current directory
		for _, fileName := range m.configFileNames {
			candidate := filepath.Join(".", fileName)
			if _, err := os.Stat(candidate); err == nil {
				filePath = candidate
				break
			}
		}
	}

	if filePath == "" {
		return nil, nil // No config file found
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, nil
	}

	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filePath, err)
	}

	var config types.ConfigFile

	// Determine file format and parse
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".json":
		if err := json.Unmarshal(content, &config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config file %s: %w", filePath, err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(content, &config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config file %s: %w", filePath, err)
		}
	default:
		// Try JSON first, then YAML
		if err := json.Unmarshal(content, &config); err != nil {
			if err := yaml.Unmarshal(content, &config); err != nil {
				return nil, fmt.Errorf("failed to parse config file %s as JSON or YAML: %w", filePath, err)
			}
		}
	}

	return &config, nil
}

// loadEnvironmentConfig loads configuration from environment variables
func (m *Manager) loadEnvironmentConfig() *types.ResolvedConfig {
	config := &types.ResolvedConfig{}

	// Swagger paths and URLs
	if paths := os.Getenv("WX_MCP_PATHS"); paths != "" {
		config.SwaggerPaths = strings.Split(paths, ",")
		for i := range config.SwaggerPaths {
			config.SwaggerPaths[i] = strings.TrimSpace(config.SwaggerPaths[i])
		}
	}

	if urls := os.Getenv("WX_MCP_URLS"); urls != "" {
		config.SwaggerURLs = strings.Split(urls, ",")
		for i := range config.SwaggerURLs {
			config.SwaggerURLs[i] = strings.TrimSpace(config.SwaggerURLs[i])
		}
	}

	// Package IDs
	if packageIDs := os.Getenv("WX_MCP_PACKAGE_ID"); packageIDs != "" {
		config.PackageIDs = strings.Split(packageIDs, ",")
		for i := range config.PackageIDs {
			config.PackageIDs[i] = strings.TrimSpace(config.PackageIDs[i])
		}
	}

	// TWC filters
	twcFilters := &types.TWCFilters{}
	hasTWCFilters := false

	if portfolios := os.Getenv("WX_MCP_TWC_PORTFOLIO"); portfolios != "" {
		twcFilters.Portfolios = strings.Split(portfolios, ",")
		for i := range twcFilters.Portfolios {
			twcFilters.Portfolios[i] = strings.TrimSpace(twcFilters.Portfolios[i])
		}
		hasTWCFilters = true
	}

	if domains := os.Getenv("WX_MCP_TWC_DOMAIN"); domains != "" {
		twcFilters.Domains = strings.Split(domains, ",")
		for i := range twcFilters.Domains {
			twcFilters.Domains[i] = strings.TrimSpace(twcFilters.Domains[i])
		}
		hasTWCFilters = true
	}

	if usages := os.Getenv("WX_MCP_TWC_USAGE"); usages != "" {
		twcFilters.UsageClassifications = strings.Split(usages, ",")
		for i := range twcFilters.UsageClassifications {
			twcFilters.UsageClassifications[i] = strings.TrimSpace(twcFilters.UsageClassifications[i])
		}
		hasTWCFilters = true
	}

	if geographies := os.Getenv("WX_MCP_TWC_GEOGRAPHY"); geographies != "" {
		twcFilters.Geographies = strings.Split(geographies, ",")
		for i := range twcFilters.Geographies {
			twcFilters.Geographies[i] = strings.TrimSpace(twcFilters.Geographies[i])
		}
		hasTWCFilters = true
	}

	if hasTWCFilters {
		config.TWCFilters = twcFilters
	}

	// Dynamic filters from WX_MCP_FILTER_* environment variables
	dynamicFilters := make(map[string]interface{})
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "WX_MCP_FILTER_") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				key := strings.ToLower(strings.TrimPrefix(parts[0], "WX_MCP_FILTER_"))
				values := strings.Split(parts[1], ",")
				for i := range values {
					values[i] = strings.TrimSpace(values[i])
				}
				if len(values) == 1 {
					dynamicFilters[key] = values[0]
				} else {
					dynamicFilters[key] = values
				}
			}
		}
	}
	if len(dynamicFilters) > 0 {
		config.DynamicFilters = dynamicFilters
	}

	// Authentication
	if apiKey := os.Getenv("WX_MCP_API_KEY"); apiKey != "" {
		config.Auth.APIKey = apiKey
	}

	// Debug
	if debug := os.Getenv("WX_MCP_DEBUG"); debug != "" {
		config.Debug = strings.ToLower(debug) == "true"
	}

	// Server configuration
	if timeout := os.Getenv("WX_MCP_TIMEOUT"); timeout != "" {
		if t, err := strconv.Atoi(timeout); err == nil {
			config.Server.Timeout = time.Duration(t) * time.Millisecond
		}
	}

	if maxTools := os.Getenv("WX_MCP_MAX_TOOLS"); maxTools != "" {
		if mt, err := strconv.Atoi(maxTools); err == nil {
			config.Server.MaxTools = mt
		}
	}

	// Logging
	if logLevel := os.Getenv("WX_MCP_LOG_LEVEL"); logLevel != "" {
		validLevels := []string{"error", "warn", "info", "debug"}
		for _, level := range validLevels {
			if strings.ToLower(logLevel) == level {
				config.Logging.Level = level
				break
			}
		}
	}

	// Swagger processing
	if validateDocs := os.Getenv("WX_MCP_VALIDATE_DOCUMENTS"); validateDocs != "" {
		config.SwaggerProcessing.ValidateDocuments = strings.ToLower(validateDocs) == "true"
	}

	if resolveRefs := os.Getenv("WX_MCP_RESOLVE_REFERENCES"); resolveRefs != "" {
		config.SwaggerProcessing.ResolveReferences = strings.ToLower(resolveRefs) == "true"
	}

	if ignoreErrors := os.Getenv("WX_MCP_IGNORE_ERRORS"); ignoreErrors != "" {
		config.SwaggerProcessing.IgnoreErrors = strings.ToLower(ignoreErrors) == "true"
	}

	return config
}

// mergeConfig merges a config file into the resolved config
func (m *Manager) mergeConfig(base *types.ResolvedConfig, override *types.ConfigFile) *types.ResolvedConfig {
	if override.Name != "" {
		base.Name = override.Name
	}
	if override.Version != "" {
		base.Version = override.Version
	}
	if len(override.SwaggerPaths) > 0 {
		base.SwaggerPaths = override.SwaggerPaths
	}
	if len(override.SwaggerURLs) > 0 {
		base.SwaggerURLs = override.SwaggerURLs
	}
	if len(override.PackageIDs) > 0 {
		base.PackageIDs = override.PackageIDs
	}
	if override.TWCFilters != nil {
		base.TWCFilters = override.TWCFilters
	}
	if override.DynamicFilters != nil {
		base.DynamicFilters = override.DynamicFilters
	}
	if override.Server != nil {
		if override.Server.Timeout > 0 {
			base.Server.Timeout = override.Server.Timeout
		}
		if override.Server.MaxTools > 0 {
			base.Server.MaxTools = override.Server.MaxTools
		}
	}
	if override.HTTP != nil {
		if override.HTTP.Timeout > 0 {
			base.HTTP.Timeout = override.HTTP.Timeout
		}
		if override.HTTP.Retries >= 0 {
			base.HTTP.Retries = override.HTTP.Retries
		}
		if override.HTTP.UserAgent != "" {
			base.HTTP.UserAgent = override.HTTP.UserAgent
		}
	}
	if override.Auth != nil {
		if override.Auth.APIKey != "" {
			base.Auth.APIKey = override.Auth.APIKey
		}
		if override.Auth.DefaultScheme != "" {
			base.Auth.DefaultScheme = override.Auth.DefaultScheme
		}
		if override.Auth.Credentials != nil {
			base.Auth.Credentials = override.Auth.Credentials
		}
	}
	if override.Debug {
		base.Debug = override.Debug
	}
	if override.Logging != nil {
		if override.Logging.Level != "" {
			base.Logging.Level = override.Logging.Level
		}
		base.Logging.Enabled = override.Logging.Enabled
	}
	if override.ToolGeneration != nil {
		base.ToolGeneration.IncludeDeprecated = override.ToolGeneration.IncludeDeprecated
		if override.ToolGeneration.MaxDescriptionLength > 0 {
			base.ToolGeneration.MaxDescriptionLength = override.ToolGeneration.MaxDescriptionLength
		}
		base.ToolGeneration.UseOperationID = override.ToolGeneration.UseOperationID
		if override.ToolGeneration.TagPrefix != "" {
			base.ToolGeneration.TagPrefix = override.ToolGeneration.TagPrefix
		}
	}
	if override.SwaggerProcessing != nil {
		base.SwaggerProcessing.ValidateDocuments = override.SwaggerProcessing.ValidateDocuments
		base.SwaggerProcessing.ResolveReferences = override.SwaggerProcessing.ResolveReferences
		base.SwaggerProcessing.IgnoreErrors = override.SwaggerProcessing.IgnoreErrors
	}
	if override.Prompts != nil {
		base.Prompts.Enabled = override.Prompts.Enabled
		base.Prompts.IncludeExamples = override.Prompts.IncludeExamples
		base.Prompts.GenerateFromEndpoints = override.Prompts.GenerateFromEndpoints
		if len(override.Prompts.Categories) > 0 {
			base.Prompts.Categories = override.Prompts.Categories
		}
	}
	if override.Resources != nil {
		base.Resources.Enabled = override.Resources.Enabled
		base.Resources.ExposeSwaggerDocs = override.Resources.ExposeSwaggerDocs
		base.Resources.EnableDocumentationSearch = override.Resources.EnableDocumentationSearch
		base.Resources.AllowEndpointDiscovery = override.Resources.AllowEndpointDiscovery
	}

	return base
}

// mergeOverrides merges override config into the resolved config
func (m *Manager) mergeOverrides(base *types.ResolvedConfig, override *types.ResolvedConfig) *types.ResolvedConfig {
	if override.Name != "" {
		base.Name = override.Name
	}
	if override.Version != "" {
		base.Version = override.Version
	}
	if len(override.SwaggerPaths) > 0 {
		base.SwaggerPaths = override.SwaggerPaths
	}
	if len(override.SwaggerURLs) > 0 {
		base.SwaggerURLs = override.SwaggerURLs
	}
	if len(override.PackageIDs) > 0 {
		base.PackageIDs = override.PackageIDs
	}
	if override.TWCFilters != nil {
		base.TWCFilters = override.TWCFilters
	}
	if override.DynamicFilters != nil {
		base.DynamicFilters = override.DynamicFilters
	}
	if override.Server.Timeout > 0 {
		base.Server.Timeout = override.Server.Timeout
	}
	if override.Server.MaxTools > 0 {
		base.Server.MaxTools = override.Server.MaxTools
	}
	if override.HTTP.Timeout > 0 {
		base.HTTP.Timeout = override.HTTP.Timeout
	}
	if override.HTTP.Retries >= 0 {
		base.HTTP.Retries = override.HTTP.Retries
	}
	if override.HTTP.UserAgent != "" {
		base.HTTP.UserAgent = override.HTTP.UserAgent
	}
	if override.Auth.APIKey != "" {
		base.Auth.APIKey = override.Auth.APIKey
	}
	if override.Auth.DefaultScheme != "" {
		base.Auth.DefaultScheme = override.Auth.DefaultScheme
	}
	if override.Auth.Credentials != nil {
		base.Auth.Credentials = override.Auth.Credentials
	}
	if override.Debug {
		base.Debug = override.Debug
	}
	if override.Logging.Level != "" {
		base.Logging.Level = override.Logging.Level
	}
	base.Logging.Enabled = override.Logging.Enabled

	// Tool Generation configuration
	if override.ToolGeneration.IncludeDeprecated {
		base.ToolGeneration.IncludeDeprecated = override.ToolGeneration.IncludeDeprecated
	}
	if override.ToolGeneration.MaxDescriptionLength > 0 {
		base.ToolGeneration.MaxDescriptionLength = override.ToolGeneration.MaxDescriptionLength
	}
	if override.ToolGeneration.UseOperationID {
		base.ToolGeneration.UseOperationID = override.ToolGeneration.UseOperationID
	}
	if override.ToolGeneration.TagPrefix != "" {
		base.ToolGeneration.TagPrefix = override.ToolGeneration.TagPrefix
	}
	if len(override.ToolGeneration.IgnoreFormats) > 0 {
		base.ToolGeneration.IgnoreFormats = override.ToolGeneration.IgnoreFormats
	}
	if override.ToolGeneration.PreferFormat != "" {
		base.ToolGeneration.PreferFormat = override.ToolGeneration.PreferFormat
	}

	return base
}

// validateConfig validates the final configuration
func (m *Manager) validateConfig(config *types.ResolvedConfig) error {
	var errors []string

	// Validate required fields
	if config.Name == "" {
		errors = append(errors, "name must be a non-empty string")
	}

	if config.Version == "" {
		errors = append(errors, "version must be a non-empty string")
	}

	// Require at least one swagger document source
	hasSwaggerPaths := len(config.SwaggerPaths) > 0
	hasSwaggerURLs := len(config.SwaggerURLs) > 0

	if !hasSwaggerPaths && !hasSwaggerURLs {
		errors = append(errors, "at least one of swaggerPaths or swaggerUrls must be provided with a non-empty array")
	}

	// Validate swagger URLs if provided
	for _, swaggerURL := range config.SwaggerURLs {
		if _, err := url.Parse(swaggerURL); err != nil {
			errors = append(errors, fmt.Sprintf("invalid URL in swaggerUrls: %s", swaggerURL))
			break
		}
	}

	// Validate server config
	if config.Server.Timeout <= 0 {
		errors = append(errors, "server.timeout must be a positive duration")
	}
	if config.Server.MaxTools <= 0 {
		errors = append(errors, "server.maxTools must be a positive number")
	}

	// Validate HTTP config
	if config.HTTP.Timeout <= 0 {
		errors = append(errors, "http.timeout must be a positive duration")
	}
	if config.HTTP.Retries < 0 {
		errors = append(errors, "http.retries must be a non-negative number")
	}

	// Validate logging config
	validLevels := []string{"error", "warn", "info", "debug"}
	validLevel := false
	for _, level := range validLevels {
		if config.Logging.Level == level {
			validLevel = true
			break
		}
	}
	if !validLevel {
		errors = append(errors, fmt.Sprintf("logging.level must be one of: %s", strings.Join(validLevels, ", ")))
	}

	if len(errors) > 0 {
		return fmt.Errorf(strings.Join(errors, "; "))
	}

	return nil
}
