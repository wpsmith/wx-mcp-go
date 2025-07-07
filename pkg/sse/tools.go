package sse

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"swagger-docs-mcp/pkg/types"
)

// initializeTools initializes swagger documents and generates tools
func (s *SSEServer) initializeTools(ctx context.Context) error {
	s.logger.Info("Initializing swagger documents and tools")

	// Scan swagger documents
	scanResult, err := s.scanner.ScanPathsAndURLs(
		s.config.SwaggerPaths,
		s.config.SwaggerURLs,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to scan swagger documents: %w", err)
	}

	s.logger.Info("Scan complete",
		zap.Int("totalFiles", scanResult.Stats.TotalFiles),
		zap.Int("validDocuments", scanResult.Stats.ValidDocuments),
		zap.Int("errors", scanResult.Stats.Errors),
		zap.String("scanTime", scanResult.Stats.ScanTime.String()))

	// Apply filters
	documents := scanResult.Documents

	// Filter by package IDs
	if len(s.config.PackageIDs) > 0 {
		documents = s.scanner.FilterDocumentsByPackageIDs(documents, s.config.PackageIDs)
		s.logger.Debug("Filtered by package IDs", zap.Int("documentsRemaining", len(documents)))
	}

	// Filter by TWC filters
	if s.config.TWCFilters != nil {
		documents = s.scanner.FilterDocumentsByTWCFilters(documents, s.config.TWCFilters)
		s.logger.Debug("Filtered by TWC filters", zap.Int("documentsRemaining", len(documents)))
	}

	// Filter by dynamic filters
	if len(s.config.DynamicFilters) > 0 {
		documents = s.scanner.FilterDocumentsByDynamicFilters(documents, s.config.DynamicFilters)
		s.logger.Debug("Filtered by dynamic filters", zap.Int("documentsRemaining", len(documents)))
	}

	// Parse documents and generate tools
	toolCount := 0
	for _, docInfo := range documents {
		var parsedDoc *types.SwaggerDocument
		var err error

		// Use appropriate parsing method based on whether content is available
		if docInfo.IsRemote && len(docInfo.Content) > 0 {
			parsedDoc, err = s.parser.ParseDocumentWithContent(&docInfo)
		} else {
			parsedDoc, err = s.parser.ParseDocument(docInfo.FilePath)
		}

		if err != nil {
			s.logger.Error("Failed to parse document",
				zap.Error(err),
				zap.String("filePath", docInfo.FilePath),
				zap.String("title", docInfo.Title),
				zap.Int("contentSize", len(docInfo.Content)),
				zap.Bool("isRemote", docInfo.IsRemote))
			continue
		}

		// Generate tools from parsed document
		tools, err := s.generator.GenerateToolsFromDocument(parsedDoc, &docInfo)
		if err != nil {
			s.logger.Error("Failed to generate tools from document",
				zap.Error(err),
				zap.String("filePath", docInfo.FilePath),
				zap.String("title", docInfo.Title),
				zap.Int("pathCount", getPathCount(parsedDoc)),
				zap.String("version", docInfo.Version))
			continue
		}

		// Register tools
		for _, tool := range tools {
			if err := s.toolRegistry.RegisterTool(tool); err != nil {
				s.logger.Error("Failed to register tool",
					zap.Error(err),
					zap.String("toolName", tool.Name),
					zap.String("document", docInfo.Title),
					zap.String("method", tool.Endpoint.Method),
					zap.String("path", tool.Endpoint.Path),
					zap.String("operationID", tool.Endpoint.OperationID))
				// Continue processing other tools even if one fails
			} else {
				toolCount++
				s.logger.Debug("Successfully registered tool",
					zap.String("toolName", tool.Name),
					zap.String("method", tool.Endpoint.Method),
					zap.String("path", tool.Endpoint.Path),
					zap.String("document", docInfo.Title),
					zap.String("version", docInfo.Version))
			}
		}

		// Check max tools limit
		if s.config.Server.MaxTools > 0 && toolCount >= s.config.Server.MaxTools {
			s.logger.Warn("Reached maximum tool limit, stopping tool generation", zap.Int("maxTools", s.config.Server.MaxTools))
			break
		}
	}

	s.logger.Info("Tool initialization complete",
		zap.Int("documentsProcessed", len(documents)),
		zap.Int("toolsGenerated", toolCount),
		zap.Int("toolsRegistered", s.toolRegistry.GetToolCount()))

	return nil
}

// getPathCount safely gets the number of paths in a swagger document
func getPathCount(document *types.SwaggerDocument) int {
	if document.Paths == nil {
		return 0
	}
	return len(document.Paths)
}