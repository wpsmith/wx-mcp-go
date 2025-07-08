package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"swagger-docs-mcp/pkg/config"
	"swagger-docs-mcp/pkg/mcp"
	"swagger-docs-mcp/pkg/server"
	"swagger-docs-mcp/pkg/sse"
	"swagger-docs-mcp/pkg/swagger"
	"swagger-docs-mcp/pkg/types"
	"swagger-docs-mcp/pkg/utils"
	"swagger-docs-mcp/pkg/version"
)

var (
	// CLI flags
	configFile        string
	swaggerPaths      []string
	swaggerPath       []string
	swaggerURLs       []string
	swaggerURL        []string
	packageIDs        []string
	twcPortfolios     []string
	twcDomains        []string
	twcUsages         []string
	twcGeographies    []string
	apiKey            string
	debug             bool
	logLevel          string
	timeout           time.Duration
	maxTools          int
	validateDocuments bool
	resolveReferences bool
	ignoreErrors      bool
	userAgent         string
	retries           int
	sseMode           bool
	mcpHTTPMode       bool
	port              int
	showVersion       bool
	ignoreFormats     []string
	preferFormat      string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "swagger-docs-mcp",
	Short: "A Model Context Protocol and SSE server for Swagger/OpenAPI documentation",
	Long: `swagger-docs-mcp is a Go-based server that dynamically converts 
Swagger/OpenAPI documentation into executable tools. It can run as either an MCP server 
for Claude Desktop or as an SSE (Server-Sent Events) HTTP server for remote deployment.`,
	RunE: runServer,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// Configuration flags
	rootCmd.Flags().StringVarP(&configFile, "config", "c", "", "config file path")

	// Swagger document sources
	rootCmd.Flags().StringSliceVar(&swaggerPaths, "swagger-paths", []string{}, "comma-separated list of swagger document paths")
	rootCmd.Flags().StringArrayVarP(&swaggerPath, "swagger-path", "s", []string{}, "single swagger document path (can be used multiple times)")
	rootCmd.Flags().StringSliceVar(&swaggerURLs, "swagger-urls", []string{}, "comma-separated list of swagger document URLs")
	rootCmd.Flags().StringArrayVarP(&swaggerURL, "swagger-url", "u", []string{}, "single swagger document URL (can be used multiple times)")

	// Package filtering
	rootCmd.Flags().StringSliceVarP(&packageIDs, "package-ids", "P", []string{}, "comma-separated list of package IDs to filter")

	// TWC filtering
	rootCmd.Flags().StringSliceVarP(&twcPortfolios, "twc-portfolios", "T", []string{}, "comma-separated list of TWC portfolios to filter")
	rootCmd.Flags().StringSliceVarP(&twcDomains, "twc-domains", "D", []string{}, "comma-separated list of TWC domains to filter")
	rootCmd.Flags().StringSliceVarP(&twcUsages, "twc-usages", "U", []string{}, "comma-separated list of TWC usage classifications to filter")
	rootCmd.Flags().StringSliceVarP(&twcGeographies, "twc-geographies", "G", []string{}, "comma-separated list of TWC geographies to filter")

	// Authentication
	rootCmd.Flags().StringVarP(&apiKey, "api-key", "k", "", "API key for authentication")

	// Server configuration
	rootCmd.Flags().BoolVarP(&debug, "debug", "v", false, "enable verbose/debug logging")
	rootCmd.Flags().StringVarP(&logLevel, "log-level", "l", "info", "log level (error, warn, info, debug)")
	rootCmd.Flags().DurationVarP(&timeout, "timeout", "t", 30*time.Second, "server timeout")
	rootCmd.Flags().IntVarP(&maxTools, "max-tools", "m", 1000, "maximum number of tools to generate")

	// Swagger processing
	rootCmd.Flags().BoolVarP(&validateDocuments, "validate-documents", "d", false, "validate swagger documents")
	rootCmd.Flags().BoolVarP(&resolveReferences, "resolve-references", "R", true, "resolve $ref references in swagger documents")
	rootCmd.Flags().BoolVarP(&ignoreErrors, "ignore-errors", "i", true, "ignore errors in swagger documents")

	// HTTP configuration
	rootCmd.Flags().StringVarP(&userAgent, "user-agent", "a", "swagger-docs-mcp/1.0.0", "HTTP user agent")
	rootCmd.Flags().IntVarP(&retries, "retries", "r", 3, "number of HTTP retries")

	// Server mode
	rootCmd.Flags().BoolVar(&sseMode, "sse", false, "run as SSE server instead of MCP server")
	rootCmd.Flags().BoolVarP(&mcpHTTPMode, "mcp-http", "H", false, "run as MCP HTTP server instead of stdio MCP server")
	rootCmd.Flags().IntVarP(&port, "port", "p", 8080, "port for SSE/MCP HTTP server")
	
	// Format filtering
	rootCmd.Flags().StringSliceVar(&ignoreFormats, "ignore-formats", []string{}, "comma-separated list of formats to ignore (e.g., xml,yaml)")
	rootCmd.Flags().StringVar(&preferFormat, "prefer-format", "", "preferred format when multiple formats exist (e.g., json, xml)")
	
	// Version flag
	rootCmd.Flags().BoolVar(&showVersion, "version", false, "show version information and exit")
}

// runServer runs the server in MCP or SSE mode
func runServer(cmd *cobra.Command, args []string) error {
	// Handle version flag
	if showVersion {
		fmt.Printf("swagger-docs-mcp %s\n", version.GetVersionWithBuildInfo())
		return nil
	}
	
	// Create configuration manager
	configManager := config.NewManager()

	// Build overrides from CLI flags
	overrides := buildConfigOverrides(cmd)

	// Load configuration
	var resolvedConfig *types.ResolvedConfig
	var err error

	if configFile != "" {
		resolvedConfig, err = configManager.LoadFromFile(configFile, overrides)
	} else {
		resolvedConfig, err = configManager.Load(overrides)
	}

	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create logger
	logger := utils.NewLogger(resolvedConfig.Logging)
	defer func() {
		_ = logger.Close() // Ignore close errors as they're typically harmless
	}()

	if debug || resolvedConfig.Debug {
		logger.UpdateConfig(types.LoggingConfig{
			Enabled: true,
			Level:   "debug",
		})
	}

	serverMode := "MCP"
	if sseMode {
		serverMode = "SSE"
	} else if mcpHTTPMode {
		serverMode = "MCP-HTTP"
	}

	logger.Info("Starting swagger-docs server",
		zap.String("mode", serverMode),
		zap.String("name", resolvedConfig.Name),
		zap.String("version", resolvedConfig.Version),
		zap.Strings("swaggerPaths", resolvedConfig.SwaggerPaths),
		zap.Strings("swaggerUrls", resolvedConfig.SwaggerURLs),
		zap.Bool("debug", resolvedConfig.Debug),
	)

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create appropriate server based on mode
	if sseMode {
		return runSSEServer(ctx, resolvedConfig, logger)
	} else if mcpHTTPMode {
		return runMCPHTTPServer(ctx, resolvedConfig, logger)
	} else {
		return runMCPServer(ctx, resolvedConfig, logger)
	}
}

// runSSEServer runs the SSE server
func runSSEServer(ctx context.Context, config *types.ResolvedConfig, logger *utils.Logger) error {
	sseServer := sse.NewSSEServer(config, logger)
	
	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- sseServer.Start(ctx)
	}()

	// Wait for shutdown signal or server error
	select {
	case sig := <-sigChan:
		logger.Info("Received signal, shutting down SSE server...", zap.String("signal", sig.String()))
		sseServer.Stop()
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("SSE server error: %w", err)
		}
	}

	logger.Info("SSE server shutdown complete")
	return nil
}

// runMCPServer runs the original MCP server (stdio)
func runMCPServer(ctx context.Context, config *types.ResolvedConfig, logger *utils.Logger) error {
	mcpServer := server.NewMCPServer(config, logger)
	
	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- mcpServer.Start(ctx)
	}()

	// Wait for shutdown signal or server error
	select {
	case sig := <-sigChan:
		logger.Info("Received signal, shutting down MCP server...", zap.String("signal", sig.String()))
		mcpServer.Stop()
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("MCP server error: %w", err)
		}
	}

	logger.Info("MCP server shutdown complete")
	return nil
}

// runMCPHTTPServer runs the new MCP HTTP server
func runMCPHTTPServer(ctx context.Context, config *types.ResolvedConfig, logger *utils.Logger) error {
	mcpServer, err := mcp.NewSimpleMCPServer(config, logger)
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	// Initialize tools from swagger documents
	err = initializeSimpleMCPTools(mcpServer, config, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize MCP tools: %w", err)
	}

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start HTTP server
	addr := fmt.Sprintf(":%d", config.Server.Port)
	
	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- mcpServer.StartHTTP(ctx, addr)
	}()

	// Wait for shutdown signal or server error
	select {
	case sig := <-sigChan:
		logger.Info("Received signal, shutting down MCP HTTP server...", zap.String("signal", sig.String()))
		// Context cancellation will stop the HTTP server
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("MCP HTTP server error: %w", err)
		}
	}

	logger.Info("MCP HTTP server shutdown complete")
	return nil
}

// initializeSimpleMCPTools scans swagger documents and registers them as MCP tools
func initializeSimpleMCPTools(mcpServer *mcp.SimpleMCPServer, config *types.ResolvedConfig, logger *utils.Logger) error {
	// Import swagger scanning and generation logic
	scanner := swagger.NewScanner(logger)
	parser := swagger.NewParser(logger)
	generator := swagger.NewToolGeneratorWithConfig(logger, &config.ToolGeneration)

	// Scan swagger documents
	scanResult, err := scanner.ScanPaths(config.SwaggerPaths, types.DefaultScanOptions())
	if err != nil {
		return fmt.Errorf("failed to scan swagger documents: %w", err)
	}

	logger.Info("Swagger document scan complete",
		zap.Int("totalFiles", scanResult.Stats.TotalFiles),
		zap.Int("validDocuments", scanResult.Stats.ValidDocuments),
		zap.Int("errors", scanResult.Stats.Errors))

	toolCount := 0
	for _, docInfo := range scanResult.Documents {
		logger.Debug("Processing swagger document", zap.String("filePath", docInfo.FilePath))

		// Parse swagger document
		swaggerDoc, err := parser.ParseDocumentWithContent(&docInfo)
		if err != nil {
			logger.Error("Failed to parse swagger document", 
				zap.String("filePath", docInfo.FilePath),
				zap.Error(err))
			continue
		}

		// Generate tools from swagger document
		tools, err := generator.GenerateToolsFromDocument(swaggerDoc, &docInfo)
		if err != nil {
			logger.Error("Failed to generate tools from swagger document",
				zap.String("filePath", docInfo.FilePath),
				zap.Error(err))
			continue
		}

		// Register each tool with MCP server
		for _, tool := range tools {
			err = mcpServer.AddSwaggerTool(tool)
			if err != nil {
				logger.Error("Failed to register MCP tool",
					zap.String("toolName", tool.Name),
					zap.Error(err))
				continue
			}
			toolCount++
		}
	}

	logger.Info("MCP tool initialization complete",
		zap.Int("documentsProcessed", len(scanResult.Documents)),
		zap.Int("toolsRegistered", toolCount))

	return nil
}

// buildConfigOverrides builds configuration overrides from CLI flags
func buildConfigOverrides(cmd *cobra.Command) *types.ResolvedConfig {
	overrides := &types.ResolvedConfig{}

	// Combine swagger paths from both flags
	allSwaggerPaths := append(swaggerPaths, swaggerPath...)
	if len(allSwaggerPaths) > 0 {
		overrides.SwaggerPaths = allSwaggerPaths
	}

	// Combine swagger URLs from both flags
	allSwaggerURLs := append(swaggerURLs, swaggerURL...)
	if len(allSwaggerURLs) > 0 {
		overrides.SwaggerURLs = allSwaggerURLs
	}

	// Package filtering
	if len(packageIDs) > 0 {
		overrides.PackageIDs = packageIDs
	}

	// TWC filtering
	if len(twcPortfolios) > 0 || len(twcDomains) > 0 || len(twcUsages) > 0 || len(twcGeographies) > 0 {
		overrides.TWCFilters = &types.TWCFilters{
			Portfolios:           twcPortfolios,
			Domains:              twcDomains,
			UsageClassifications: twcUsages,
			Geographies:          twcGeographies,
		}
	}

	// Authentication
	if apiKey != "" {
		overrides.Auth.APIKey = apiKey
	}

	// Debug
	if debug {
		overrides.Debug = true
	}

	// Logging
	if logLevel != "" {
		overrides.Logging.Level = logLevel
		overrides.Logging.Enabled = true
	}

	// Server configuration
	if timeout > 0 {
		overrides.Server.Timeout = timeout
	}
	if maxTools > 0 {
		overrides.Server.MaxTools = maxTools
	}
	if port > 0 {
		overrides.Server.Port = port
	}

	// Swagger processing
	if cmd.Flags().Changed("validate-documents") {
		overrides.SwaggerProcessing.ValidateDocuments = validateDocuments
	}
	if cmd.Flags().Changed("resolve-references") {
		overrides.SwaggerProcessing.ResolveReferences = resolveReferences
	}
	if cmd.Flags().Changed("ignore-errors") {
		overrides.SwaggerProcessing.IgnoreErrors = ignoreErrors
	}

	// HTTP configuration
	if userAgent != "" {
		overrides.HTTP.UserAgent = userAgent
	}
	if retries >= 0 {
		overrides.HTTP.Retries = retries
	}

	// Format filtering
	if len(ignoreFormats) > 0 {
		overrides.ToolGeneration.IgnoreFormats = ignoreFormats
	}
	if preferFormat != "" {
		overrides.ToolGeneration.PreferFormat = preferFormat
	}

	return overrides
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number and build information",
	Long: `Print the version number and build information including build date, 
commit hash, Go version, and build user.`,
	Run: func(cmd *cobra.Command, args []string) {
		detailed, _ := cmd.Flags().GetBool("detailed")
		if detailed {
			fmt.Printf("swagger-docs-mcp %s\n", version.GetDetailedVersionString())
		} else {
			fmt.Printf("swagger-docs-mcp %s\n", version.GetVersionString())
		}
	},
}

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		configManager := config.NewManager()
		overrides := buildConfigOverrides(cmd)

		var resolvedConfig *types.ResolvedConfig
		var err error

		if configFile != "" {
			resolvedConfig, err = configManager.LoadFromFile(configFile, overrides)
		} else {
			resolvedConfig, err = configManager.Load(overrides)
		}

		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Print configuration
		fmt.Printf("Configuration:\n")
		fmt.Printf("  Name: %s\n", resolvedConfig.Name)
		fmt.Printf("  Version: %s\n", resolvedConfig.Version)
		fmt.Printf("  Debug: %t\n", resolvedConfig.Debug)
		fmt.Printf("  Log Level: %s\n", resolvedConfig.Logging.Level)
		fmt.Printf("  Swagger Paths: %s\n", strings.Join(resolvedConfig.SwaggerPaths, ", "))
		fmt.Printf("  Swagger URLs: %s\n", strings.Join(resolvedConfig.SwaggerURLs, ", "))

		if len(resolvedConfig.PackageIDs) > 0 {
			fmt.Printf("  Package IDs: %s\n", strings.Join(resolvedConfig.PackageIDs, ", "))
		}

		if resolvedConfig.TWCFilters != nil {
			fmt.Printf("  TWC Filters:\n")
			if len(resolvedConfig.TWCFilters.Portfolios) > 0 {
				fmt.Printf("    Portfolios: %s\n", strings.Join(resolvedConfig.TWCFilters.Portfolios, ", "))
			}
			if len(resolvedConfig.TWCFilters.Domains) > 0 {
				fmt.Printf("    Domains: %s\n", strings.Join(resolvedConfig.TWCFilters.Domains, ", "))
			}
			if len(resolvedConfig.TWCFilters.UsageClassifications) > 0 {
				fmt.Printf("    Usage Classifications: %s\n", strings.Join(resolvedConfig.TWCFilters.UsageClassifications, ", "))
			}
			if len(resolvedConfig.TWCFilters.Geographies) > 0 {
				fmt.Printf("    Geographies: %s\n", strings.Join(resolvedConfig.TWCFilters.Geographies, ", "))
			}
		}

		fmt.Printf("  Server:\n")
		fmt.Printf("    Timeout: %s\n", resolvedConfig.Server.Timeout.String())
		fmt.Printf("    Max Tools: %d\n", resolvedConfig.Server.MaxTools)

		fmt.Printf("  HTTP:\n")
		fmt.Printf("    Timeout: %s\n", resolvedConfig.HTTP.Timeout.String())
		fmt.Printf("    Retries: %d\n", resolvedConfig.HTTP.Retries)
		fmt.Printf("    User Agent: %s\n", resolvedConfig.HTTP.UserAgent)

		fmt.Printf("  Tool Generation:\n")
		fmt.Printf("    Include Deprecated: %t\n", resolvedConfig.ToolGeneration.IncludeDeprecated)
		if len(resolvedConfig.ToolGeneration.IgnoreFormats) > 0 {
			fmt.Printf("    Ignore Formats: %s\n", strings.Join(resolvedConfig.ToolGeneration.IgnoreFormats, ", "))
		}
		if resolvedConfig.ToolGeneration.PreferFormat != "" {
			fmt.Printf("    Prefer Format: %s\n", resolvedConfig.ToolGeneration.PreferFormat)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(configCmd)

	// Add flags to version command
	versionCmd.Flags().BoolP("detailed", "d", false, "show detailed version information")

	// Add global flags to config command
	configCmd.Flags().AddFlagSet(rootCmd.Flags())
}
