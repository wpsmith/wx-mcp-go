package swagger

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"swagger-docs-mcp/pkg/types"
	"swagger-docs-mcp/pkg/utils"
)

// Scanner handles swagger document discovery and scanning
type Scanner struct {
	logger         *utils.Logger
	defaultOptions *types.ScanOptions
}

// NewScanner creates a new swagger document scanner
func NewScanner(logger *utils.Logger) *Scanner {
	return &Scanner{
		logger:         logger.Child("scanner"),
		defaultOptions: types.DefaultScanOptions(),
	}
}

// ScanPaths scans multiple paths for swagger documents
func (s *Scanner) ScanPaths(paths []string, options *types.ScanOptions) (*types.ScanResult, error) {
	startTime := time.Now()
	resolvedOptions := s.defaultOptions
	if options != nil {
		resolvedOptions = options
	}

	s.logger.Info("Starting swagger document scan",
		zap.Strings("paths", paths),
		zap.Any("options", resolvedOptions))

	allDocuments := []types.SwaggerDocumentInfo{}
	allErrors := []types.ScanError{}
	totalFiles := 0

	for _, path := range paths {
		result, err := s.scanSinglePath(path, resolvedOptions)
		if err != nil {
			s.logger.Error("Failed to scan path", zap.String("path", path), zap.Error(err))
			allErrors = append(allErrors, types.ScanError{
				Path:  path,
				Error: err.Error(),
			})
			continue
		}
		allDocuments = append(allDocuments, result.Documents...)
		allErrors = append(allErrors, result.Errors...)
		totalFiles += result.Stats.TotalFiles
	}

	scanTime := time.Since(startTime)
	stats := types.ScanStats{
		TotalFiles:     totalFiles,
		ValidDocuments: len(allDocuments),
		Errors:         len(allErrors),
		ScanTime:       scanTime,
	}

	s.logger.Info("Swagger document scan complete",
		zap.Int("totalFiles", stats.TotalFiles),
		zap.Int("validDocuments", stats.ValidDocuments),
		zap.Int("errors", stats.Errors),
		zap.String("scanTime", stats.ScanTime.String()))

	return &types.ScanResult{
		Documents: allDocuments,
		Errors:    allErrors,
		Stats:     stats,
	}, nil
}

// ScanPathsAndURLs scans both local paths and remote URLs
func (s *Scanner) ScanPathsAndURLs(paths []string, urls []string, options *types.ScanOptions) (*types.ScanResult, error) {
	startTime := time.Now()
	resolvedOptions := s.defaultOptions
	if options != nil {
		resolvedOptions = options
	}

	s.logger.Info("Starting swagger document scan",
		zap.Strings("paths", paths),
		zap.Strings("urls", urls),
		zap.Any("options", resolvedOptions))

	allDocuments := []types.SwaggerDocumentInfo{}
	allErrors := []types.ScanError{}
	totalFiles := 0

	// Scan local paths
	for _, path := range paths {
		result, err := s.scanSinglePath(path, resolvedOptions)
		if err != nil {
			s.logger.Error("Failed to scan path", zap.String("path", path), zap.Error(err))
			allErrors = append(allErrors, types.ScanError{
				Path:  path,
				Error: err.Error(),
			})
			continue
		}
		allDocuments = append(allDocuments, result.Documents...)
		allErrors = append(allErrors, result.Errors...)
		totalFiles += result.Stats.TotalFiles
	}

	// Scan remote URLs
	for _, u := range urls {
		result, err := s.scanSingleURL(u)
		if err != nil {
			s.logger.Error("Failed to scan URL", zap.String("url", u), zap.Error(err))
			allErrors = append(allErrors, types.ScanError{
				Path:  u,
				Error: err.Error(),
			})
			continue
		}
		allDocuments = append(allDocuments, result.Documents...)
		allErrors = append(allErrors, result.Errors...)
		totalFiles += result.Stats.TotalFiles
	}

	scanTime := time.Since(startTime)
	stats := types.ScanStats{
		TotalFiles:     totalFiles,
		ValidDocuments: len(allDocuments),
		Errors:         len(allErrors),
		ScanTime:       scanTime,
	}

	s.logger.Info("Swagger document scan complete",
		zap.Int("totalFiles", stats.TotalFiles),
		zap.Int("validDocuments", stats.ValidDocuments),
		zap.Int("errors", stats.Errors),
		zap.String("scanTime", stats.ScanTime.String()))

	return &types.ScanResult{
		Documents: allDocuments,
		Errors:    allErrors,
		Stats:     stats,
	}, nil
}

// scanSinglePath scans a single path for swagger documents
func (s *Scanner) scanSinglePath(path string, options *types.ScanOptions) (*types.ScanResult, error) {
	s.logger.Debug("Scanning path", zap.String("path", path))

	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for '%s': %w", path, err)
	}

	// Check if path exists
	stat, err := os.Stat(absPath)
	if err != nil {
		return &types.ScanResult{
			Documents: []types.SwaggerDocumentInfo{},
			Errors: []types.ScanError{{
				Path:  path,
				Error: err.Error(),
			}},
			Stats: types.ScanStats{
				TotalFiles:     0,
				ValidDocuments: 0,
				Errors:         1,
				ScanTime:       0,
			},
		}, nil
	}

	if stat.IsDir() {
		return s.scanDirectory(absPath, options)
	} else {
		return s.scanSingleFile(absPath)
	}
}

// scanDirectory scans a directory for swagger documents
func (s *Scanner) scanDirectory(dirPath string, options *types.ScanOptions) (*types.ScanResult, error) {
	s.logger.Debug("Scanning directory", zap.String("dirPath", dirPath))

	documents := []types.SwaggerDocumentInfo{}
	errors := []types.ScanError{}

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walking
		}

		if info.IsDir() {
			// Check depth limit
			relPath, _ := filepath.Rel(dirPath, path)
			depth := len(strings.Split(relPath, string(os.PathSeparator)))
			if depth > options.MaxDepth {
				return filepath.SkipDir
			}
			return nil
		}

		// Check file extension
		ext := strings.ToLower(filepath.Ext(path))
		validExt := false
		for _, supportedExt := range options.SupportedExtensions {
			if ext == supportedExt {
				validExt = true
				break
			}
		}

		if !validExt {
			return nil
		}

		// Scan the file
		result, err := s.scanSingleFile(path)
		if err != nil {
			errors = append(errors, types.ScanError{
				Path:  path,
				Error: err.Error(),
			})
		} else {
			documents = append(documents, result.Documents...)
			errors = append(errors, result.Errors...)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory '%s': %w", dirPath, err)
	}

	return &types.ScanResult{
		Documents: documents,
		Errors:    errors,
		Stats: types.ScanStats{
			TotalFiles:     len(documents) + len(errors),
			ValidDocuments: len(documents),
			Errors:         len(errors),
			ScanTime:       0,
		},
	}, nil
}

// scanSingleFile scans a single file
func (s *Scanner) scanSingleFile(filePath string) (*types.ScanResult, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	// Check if supported extension
	validExt := false
	for _, supportedExt := range s.defaultOptions.SupportedExtensions {
		if ext == supportedExt {
			validExt = true
			break
		}
	}

	if !validExt {
		return &types.ScanResult{
			Documents: []types.SwaggerDocumentInfo{},
			Errors: []types.ScanError{{
				Path:  filePath,
				Error: fmt.Sprintf("Unsupported file extension: %s", ext),
			}},
			Stats: types.ScanStats{
				TotalFiles:     1,
				ValidDocuments: 0,
				Errors:         1,
				ScanTime:       0,
			},
		}, nil
	}

	// Extract version from file path
	version := s.extractVersionFromPath(filePath)

	// Extract document metadata
	metadata, err := s.extractDocumentMetadata(filePath, ext)
	if err != nil {
		return &types.ScanResult{
			Documents: []types.SwaggerDocumentInfo{},
			Errors: []types.ScanError{{
				Path:  filePath,
				Error: fmt.Sprintf("Failed to scan file: %s", err.Error()),
			}},
			Stats: types.ScanStats{
				TotalFiles:     1,
				ValidDocuments: 0,
				Errors:         1,
				ScanTime:       0,
			},
		}, nil
	}

	documentInfo := types.SwaggerDocumentInfo{
		FilePath:  filePath,
		Version:   version,
		Title:     strings.TrimSuffix(filepath.Base(filePath), ext),
		Endpoints: []types.SwaggerEndpoint{}, // Will be populated during parsing
	}

	// Copy metadata
	if metadata.PackageIDs != nil {
		documentInfo.PackageIDs = metadata.PackageIDs
	}
	if metadata.TwcDomainPortfolio != nil {
		documentInfo.TwcDomainPortfolio = metadata.TwcDomainPortfolio
	}
	if metadata.TwcDomain != nil {
		documentInfo.TwcDomain = metadata.TwcDomain
	}
	if metadata.TwcUsageClassification != nil {
		documentInfo.TwcUsageClassification = metadata.TwcUsageClassification
	}
	if metadata.TwcGeography != nil {
		documentInfo.TwcGeography = metadata.TwcGeography
	}

	return &types.ScanResult{
		Documents: []types.SwaggerDocumentInfo{documentInfo},
		Errors:    []types.ScanError{},
		Stats: types.ScanStats{
			TotalFiles:     1,
			ValidDocuments: 1,
			Errors:         0,
			ScanTime:       0,
		},
	}, nil
}

// scanSingleURL scans a single remote URL for swagger document
func (s *Scanner) scanSingleURL(rawURL string) (*types.ScanResult, error) {
	s.logger.Debug("Scanning URL", zap.String("url", rawURL))

	// Validate URL format
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL '%s': %w", rawURL, err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported protocol '%s' in URL '%s' - only HTTP/HTTPS supported", parsedURL.Scheme, rawURL)
	}

	// Fetch the document
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request for URL '%s': %w", rawURL, err)
	}

	req.Header.Set("Accept", "application/json, application/yaml, text/yaml, */*")
	req.Header.Set("User-Agent", "swagger-docs-mcp/1.0.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL '%s' (timeout: 30s): %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s for URL '%s' (content-type: %s)", resp.StatusCode, resp.Status, rawURL, resp.Header.Get("Content-Type"))
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from URL '%s' (status: %d, content-length: %s): %w", rawURL, resp.StatusCode, resp.Header.Get("Content-Length"), err)
	}

	// Determine format from content type or URL extension
	contentType := resp.Header.Get("Content-Type")
	isYAML := strings.Contains(contentType, "yaml") ||
		strings.Contains(contentType, "yml") ||
		strings.HasSuffix(rawURL, ".yaml") ||
		strings.HasSuffix(rawURL, ".yml")

	// Parse the content first to check if it's an array of URLs
	var parsedContent interface{}
	if isYAML {
		err = yaml.Unmarshal(content, &parsedContent)
	} else {
		err = json.Unmarshal(content, &parsedContent)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse swagger document from URL '%s' (content size: %d bytes): %w", rawURL, len(content), err)
	}

	// Check if the content is an array of URLs
	if urlArray, ok := parsedContent.([]interface{}); ok {
		s.logger.Debug("URL contains array of URLs, processing each...", zap.Int("urlCount", len(urlArray)))
		return s.processURLArray(urlArray, rawURL)
	}

	// Otherwise, treat as a regular swagger document
	document, ok := parsedContent.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("document from URL '%s' is not a valid JSON/YAML object (content preview: %.100s...)", rawURL, string(content))
	}

	// Extract version from URL or document
	version := s.extractVersionFromURL(rawURL)
	if version == "" {
		version = s.extractVersionFromDocument(document)
	}

	// Create a unique title from URL
	title := s.createTitleFromURL(rawURL)

	// Extract metadata from document
	metadata := s.extractMetadataFromDocument(document)

	documentInfo := types.SwaggerDocumentInfo{
		FilePath:  rawURL, // Use URL as file path for remote documents
		Version:   version,
		Title:     title,
		Endpoints: []types.SwaggerEndpoint{}, // Will be populated during parsing
		IsRemote:  true,
		Content:   content, // Store the fetched content
	}

	// Copy metadata
	if metadata.PackageIDs != nil {
		documentInfo.PackageIDs = metadata.PackageIDs
	}
	if metadata.TwcDomainPortfolio != nil {
		documentInfo.TwcDomainPortfolio = metadata.TwcDomainPortfolio
	}
	if metadata.TwcDomain != nil {
		documentInfo.TwcDomain = metadata.TwcDomain
	}
	if metadata.TwcUsageClassification != nil {
		documentInfo.TwcUsageClassification = metadata.TwcUsageClassification
	}
	if metadata.TwcGeography != nil {
		documentInfo.TwcGeography = metadata.TwcGeography
	}

	s.logger.Debug("Successfully scanned URL",
		zap.String("url", rawURL),
		zap.String("version", version),
		zap.String("title", title),
		zap.Any("metadata", metadata))

	return &types.ScanResult{
		Documents: []types.SwaggerDocumentInfo{documentInfo},
		Errors:    []types.ScanError{},
		Stats: types.ScanStats{
			TotalFiles:     1,
			ValidDocuments: 1,
			Errors:         0,
			ScanTime:       0,
		},
	}, nil
}

// processURLArray processes an array of URLs from a URL list document concurrently
func (s *Scanner) processURLArray(urlArray []interface{}, sourceURL string) (*types.ScanResult, error) {
	s.logger.Info(fmt.Sprintf("Processing URL array from %s with %d entries", sourceURL, len(urlArray)))

	// Validate URLs first and collect valid ones
	var validURLs []string
	var initialErrors []types.ScanError

	for _, item := range urlArray {
		// Validate that each item is a string (URL)
		urlStr, ok := item.(string)
		if !ok {
			initialErrors = append(initialErrors, types.ScanError{
				Path:  sourceURL,
				Error: fmt.Sprintf("Invalid URL in array: expected string, got %T", item),
			})
			continue
		}

		// Validate URL format
		if _, err := url.Parse(urlStr); err != nil {
			initialErrors = append(initialErrors, types.ScanError{
				Path:  urlStr,
				Error: fmt.Sprintf("Invalid URL format: %s", err.Error()),
			})
			continue
		}

		validURLs = append(validURLs, urlStr)
	}

	// If no valid URLs, return early
	if len(validURLs) == 0 {
		return &types.ScanResult{
			Documents: []types.SwaggerDocumentInfo{},
			Errors:    initialErrors,
			Stats: types.ScanStats{
				TotalFiles:     0,
				ValidDocuments: 0,
				Errors:         len(initialErrors),
				ScanTime:       0,
			},
		}, nil
	}

	// Process URLs concurrently
	type urlResult struct {
		documents []types.SwaggerDocumentInfo
		errors    []types.ScanError
		files     int
	}

	resultChan := make(chan urlResult, len(validURLs))
	var wg sync.WaitGroup

	// Launch goroutines for each valid URL
	for _, urlStr := range validURLs {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			s.logger.Debug("Processing URL from array concurrently", zap.String("url", url))

			// Recursively scan each URL
			result, err := s.scanSingleURL(url)

			if err != nil {
				s.logger.Error("Failed to process URL from array", zap.String("url", url), zap.Error(err))
				resultChan <- urlResult{
					documents: []types.SwaggerDocumentInfo{},
					errors: []types.ScanError{{
						Path:  url,
						Error: fmt.Sprintf("Failed to process URL: %s", err.Error()),
					}},
					files: 0,
				}
			} else {
				resultChan <- urlResult{
					documents: result.Documents,
					errors:    result.Errors,
					files:     result.Stats.TotalFiles,
				}
			}
		}(urlStr)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect all results
	allDocuments := []types.SwaggerDocumentInfo{}
	allErrors := initialErrors
	totalFiles := 0

	for result := range resultChan {
		allDocuments = append(allDocuments, result.documents...)
		allErrors = append(allErrors, result.errors...)
		totalFiles += result.files
	}

	s.logger.Info("Completed concurrent processing of URL array",
		zap.Int("totalURLs", len(validURLs)),
		zap.Int("documentsFound", len(allDocuments)),
		zap.Int("errors", len(allErrors)-len(initialErrors)))

	return &types.ScanResult{
		Documents: allDocuments,
		Errors:    allErrors,
		Stats: types.ScanStats{
			TotalFiles:     totalFiles,
			ValidDocuments: len(allDocuments),
			Errors:         len(allErrors),
			ScanTime:       0,
		},
	}, nil
}

// extractVersionFromPath extracts API version from file path
func (s *Scanner) extractVersionFromPath(filePath string) string {
	// Look for version patterns in the path
	pathParts := strings.Split(filePath, string(os.PathSeparator))

	// Check for version directories (v1, v2, v3, etc.)
	versionRegex := regexp.MustCompile(`^v(\d+)$`)
	for _, part := range pathParts {
		if matches := versionRegex.FindStringSubmatch(part); len(matches) > 1 {
			return matches[1]
		}
	}

	// Check for version in filename
	filename := filepath.Base(filePath)
	filenameVersionRegex := regexp.MustCompile(`v(\d+)`)
	if matches := filenameVersionRegex.FindStringSubmatch(filename); len(matches) > 1 {
		return matches[1]
	}

	// Default to v1 if no version found
	return "1"
}

// extractVersionFromURL extracts version from URL
func (s *Scanner) extractVersionFromURL(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "1"
	}

	pathParts := strings.Split(parsedURL.Path, "/")

	// Check for version directories (v1, v2, v3, etc.)
	versionRegex := regexp.MustCompile(`^v(\d+)$`)
	for _, part := range pathParts {
		if matches := versionRegex.FindStringSubmatch(part); len(matches) > 1 {
			return matches[1]
		}
	}

	// Check for version in filename
	if len(pathParts) > 0 {
		filename := pathParts[len(pathParts)-1]
		filenameVersionRegex := regexp.MustCompile(`v(\d+)`)
		if matches := filenameVersionRegex.FindStringSubmatch(filename); len(matches) > 1 {
			return matches[1]
		}
	}

	// Default to v1 if no version found
	return "1"
}

// extractVersionFromDocument extracts version from swagger document
func (s *Scanner) extractVersionFromDocument(document map[string]interface{}) string {
	// Check info.version field
	if info, ok := document["info"].(map[string]interface{}); ok {
		if version, ok := info["version"].(string); ok {
			versionRegex := regexp.MustCompile(`^v?(\d+)`)
			if matches := versionRegex.FindStringSubmatch(version); len(matches) > 1 {
				return matches[1]
			}
		}
	}

	// Check OpenAPI version
	if openapi, ok := document["openapi"].(string); ok {
		openAPIVersionRegex := regexp.MustCompile(`^(\d+)`)
		if matches := openAPIVersionRegex.FindStringSubmatch(openapi); len(matches) > 1 {
			return matches[1]
		}
	}

	// Default to v1
	return "1"
}

// createTitleFromURL creates a human-readable title from URL
func (s *Scanner) createTitleFromURL(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "Remote Swagger Document"
	}

	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) == 0 {
		return parsedURL.Host
	}

	// Use the last part of the path as the base title
	title := pathParts[len(pathParts)-1]
	if title == "" {
		title = parsedURL.Host
	}

	// Remove file extension
	fileExtRegex := regexp.MustCompile(`\.(json|yaml|yml)$`)
	title = fileExtRegex.ReplaceAllString(title, "")

	// Convert to human-readable format
	title = strings.ReplaceAll(title, "-", " ")
	title = strings.ReplaceAll(title, "_", " ")

	// Capitalize words
	words := strings.Fields(title)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
	}
	title = strings.Join(words, " ")

	if title == "" {
		return "Remote Swagger Document"
	}

	return title
}

// extractDocumentMetadata extracts metadata from a swagger document file
func (s *Scanner) extractDocumentMetadata(filePath string, extension string) (*types.SwaggerDocumentInfo, error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file '%s' (size: %s): %w", filePath, getFileSize(filePath), err)
	}

	var document map[string]interface{}

	switch extension {
	case ".json":
		if err := json.Unmarshal(content, &document); err != nil {
			return nil, fmt.Errorf("failed to parse JSON file '%s' (size: %d bytes): %w", filePath, len(content), err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(content, &document); err != nil {
			return nil, fmt.Errorf("failed to parse YAML file '%s' (size: %d bytes): %w", filePath, len(content), err)
		}
	default:
		return &types.SwaggerDocumentInfo{}, nil
	}

	return s.extractMetadataFromDocument(document), nil
}

// extractMetadataFromDocument extracts metadata from a parsed swagger document
func (s *Scanner) extractMetadataFromDocument(document map[string]interface{}) *types.SwaggerDocumentInfo {
	result := &types.SwaggerDocumentInfo{}

	// Extract package IDs
	result.PackageIDs = s.extractStringArrayFromInterface(document["x-package-ids"])

	// Extract TWC domain portfolio
	result.TwcDomainPortfolio = s.extractStringArrayFromInterface(document["x-twc-domain-portfolio"])

	// Extract TWC domain
	result.TwcDomain = s.extractStringArrayFromInterface(document["x-twc-domain"])

	// Extract TWC usage classification
	result.TwcUsageClassification = s.extractStringArrayFromInterface(document["x-twc-usage-classification"])

	// Extract TWC geography
	result.TwcGeography = s.extractStringArrayFromInterface(document["x-twc-geography"])

	return result
}

// extractStringArrayFromInterface converts interface{} to []string, handling both strings and arrays
func (s *Scanner) extractStringArrayFromInterface(value interface{}) []string {
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
		s.logger.Debug("Unexpected type for extension field", zap.String("type", fmt.Sprintf("%T", v)))
		return nil
	}
}

// Filter methods for documents

// FilterDocumentsByPackageIDs filters documents by package IDs
func (s *Scanner) FilterDocumentsByPackageIDs(documents []types.SwaggerDocumentInfo, packageIDs []string) []types.SwaggerDocumentInfo {
	if len(packageIDs) == 0 {
		return documents
	}

	var filtered []types.SwaggerDocumentInfo
	for _, doc := range documents {
		if len(doc.PackageIDs) == 0 {
			continue
		}

		// Check if any of the document's package IDs match any of the filter IDs
		hasMatch := false
		for _, docID := range doc.PackageIDs {
			for _, filterID := range packageIDs {
				if docID == filterID {
					hasMatch = true
					break
				}
			}
			if hasMatch {
				break
			}
		}

		if hasMatch {
			filtered = append(filtered, doc)
		}
	}

	return filtered
}

// FilterDocumentsByTWCFilters filters documents by TWC filters
func (s *Scanner) FilterDocumentsByTWCFilters(documents []types.SwaggerDocumentInfo, twcFilters *types.TWCFilters) []types.SwaggerDocumentInfo {
	if twcFilters == nil {
		return documents
	}

	var filtered []types.SwaggerDocumentInfo
	for _, doc := range documents {
		match := true

		// Check portfolio filter
		if len(twcFilters.Portfolios) > 0 {
			if len(doc.TwcDomainPortfolio) == 0 {
				match = false
			} else {
				portfolioMatch := false
				for _, docPortfolio := range doc.TwcDomainPortfolio {
					for _, filterPortfolio := range twcFilters.Portfolios {
						if docPortfolio == filterPortfolio {
							portfolioMatch = true
							break
						}
					}
					if portfolioMatch {
						break
					}
				}
				if !portfolioMatch {
					match = false
				}
			}
		}

		// Check domain filter
		if match && len(twcFilters.Domains) > 0 {
			if len(doc.TwcDomain) == 0 {
				match = false
			} else {
				domainMatch := false
				for _, docDomain := range doc.TwcDomain {
					for _, filterDomain := range twcFilters.Domains {
						if docDomain == filterDomain {
							domainMatch = true
							break
						}
					}
					if domainMatch {
						break
					}
				}
				if !domainMatch {
					match = false
				}
			}
		}

		// Check usage classification filter
		if match && len(twcFilters.UsageClassifications) > 0 {
			if len(doc.TwcUsageClassification) == 0 {
				match = false
			} else {
				usageMatch := false
				for _, docUsage := range doc.TwcUsageClassification {
					for _, filterUsage := range twcFilters.UsageClassifications {
						if docUsage == filterUsage {
							usageMatch = true
							break
						}
					}
					if usageMatch {
						break
					}
				}
				if !usageMatch {
					match = false
				}
			}
		}

		// Check geography filter
		if match && len(twcFilters.Geographies) > 0 {
			if len(doc.TwcGeography) == 0 {
				match = false
			} else {
				geoMatch := false
				for _, docGeo := range doc.TwcGeography {
					for _, filterGeo := range twcFilters.Geographies {
						if docGeo == filterGeo {
							geoMatch = true
							break
						}
					}
					if geoMatch {
						break
					}
				}
				if !geoMatch {
					match = false
				}
			}
		}

		if match {
			filtered = append(filtered, doc)
		}
	}

	return filtered
}

// FilterDocumentsByDynamicFilters filters documents by dynamic filters
func (s *Scanner) FilterDocumentsByDynamicFilters(documents []types.SwaggerDocumentInfo, dynamicFilters map[string]interface{}) []types.SwaggerDocumentInfo {
	if len(dynamicFilters) == 0 {
		return documents
	}

	// Implementation would depend on how dynamic filters map to document fields
	// For now, return unfiltered documents
	return documents
}

// getFileSize safely gets file size as a string
func getFileSize(filePath string) string {
	if info, err := os.Stat(filePath); err == nil {
		return fmt.Sprintf("%d bytes", info.Size())
	}
	return "unknown size"
}
