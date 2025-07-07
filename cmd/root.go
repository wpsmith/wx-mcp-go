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
	"swagger-docs-mcp/pkg/server"
	"swagger-docs-mcp/pkg/sse"
	"swagger-docs-mcp/pkg/types"
	"swagger-docs-mcp/pkg/utils"
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
	port              int
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
	rootCmd.Flags().StringArrayVar(&swaggerPath, "swagger-path", []string{}, "single swagger document path (can be used multiple times)")
	rootCmd.Flags().StringSliceVar(&swaggerURLs, "swagger-urls", []string{}, "comma-separated list of swagger document URLs")
	rootCmd.Flags().StringArrayVar(&swaggerURL, "swagger-url", []string{}, "single swagger document URL (can be used multiple times)")

	// Package filtering
	rootCmd.Flags().StringSliceVar(&packageIDs, "package-ids", []string{}, "comma-separated list of package IDs to filter")

	// TWC filtering
	rootCmd.Flags().StringSliceVar(&twcPortfolios, "twc-portfolios", []string{}, "comma-separated list of TWC portfolios to filter")
	rootCmd.Flags().StringSliceVar(&twcDomains, "twc-domains", []string{}, "comma-separated list of TWC domains to filter")
	rootCmd.Flags().StringSliceVar(&twcUsages, "twc-usages", []string{}, "comma-separated list of TWC usage classifications to filter")
	rootCmd.Flags().StringSliceVar(&twcGeographies, "twc-geographies", []string{}, "comma-separated list of TWC geographies to filter")

	// Authentication
	rootCmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")

	// Server configuration
	rootCmd.Flags().BoolVar(&debug, "debug", false, "enable debug logging")
	rootCmd.Flags().StringVar(&logLevel, "log-level", "info", "log level (error, warn, info, debug)")
	rootCmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "server timeout")
	rootCmd.Flags().IntVar(&maxTools, "max-tools", 1000, "maximum number of tools to generate")

	// Swagger processing
	rootCmd.Flags().BoolVar(&validateDocuments, "validate-documents", false, "validate swagger documents")
	rootCmd.Flags().BoolVar(&resolveReferences, "resolve-references", true, "resolve $ref references in swagger documents")
	rootCmd.Flags().BoolVar(&ignoreErrors, "ignore-errors", true, "ignore errors in swagger documents")

	// HTTP configuration
	rootCmd.Flags().StringVar(&userAgent, "user-agent", "swagger-docs-mcp/1.0.0", "HTTP user agent")
	rootCmd.Flags().IntVar(&retries, "retries", 3, "number of HTTP retries")

	// Server mode
	rootCmd.Flags().BoolVar(&sseMode, "sse", false, "run as SSE server instead of MCP server")
	rootCmd.Flags().IntVar(&port, "port", 8080, "port for SSE server")
}

// runServer runs the server in MCP or SSE mode
func runServer(cmd *cobra.Command, args []string) error {
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
	}

	logger.Info("Starting swagger-docs server",
		zap.String("mode", serverMode),
		zap.String("name", resolvedConfig.Name),
		zap.String("version", resolvedConfig.Version),
		zap.Strings("swaggerPaths", resolvedConfig.SwaggerPaths),
		zap.Strings("swaggerUrls", resolvedConfig.SwaggerURLs),
		zap.Bool("debug", resolvedConfig.Debug),
	)

	// Create appropriate server based on mode
	var serverInstance interface {
		Start(context.Context) error
		Stop()
	}

	if sseMode {
		serverInstance = sse.NewSSEServer(resolvedConfig, logger)
	} else {
		serverInstance = server.NewMCPServer(resolvedConfig, logger)
	}

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- serverInstance.Start(ctx)
	}()

	// Wait for shutdown signal or server error
	select {
	case sig := <-sigChan:
		logger.Info("Received signal, shutting down...", zap.String("signal", sig.String()))
		cancel()
		serverInstance.Stop()
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("server error: %w", err)
		}
	}

	logger.Info("Server shutdown complete")
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

	return overrides
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("swagger-docs-mcp version 1.0.0\n")
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

		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(configCmd)

	// Add global flags to config command
	configCmd.Flags().AddFlagSet(rootCmd.Flags())
}
