package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
	"swagger-docs-mcp/pkg/types"
	"swagger-docs-mcp/pkg/utils"
)

// Client handles HTTP requests for API execution
type Client struct {
	config     *types.ResolvedConfig
	logger     *utils.Logger
	httpClient *http.Client
}

// Response represents an HTTP response
type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

// NewClient creates a new HTTP client
func NewClient(config *types.ResolvedConfig, logger *utils.Logger) *Client {
	httpClient := &http.Client{
		Timeout: config.HTTP.Timeout,
	}

	return &Client{
		config:     config,
		logger:     logger.Child("http-client"),
		httpClient: httpClient,
	}
}

// ExecuteRequest executes an HTTP request for a swagger endpoint
func (c *Client) ExecuteRequest(endpoint *types.SwaggerEndpoint, arguments map[string]interface{}) (*Response, error) {
	c.logger.Debug("Executing request", zap.String("method", endpoint.Method), zap.String("path", endpoint.Path), zap.Any("arguments", arguments))

	// Build the request
	req, err := c.buildRequest(endpoint, arguments)
	if err != nil {
		return nil, fmt.Errorf("failed to build HTTP request for %s %s (args: %v): %w", endpoint.Method, endpoint.Path, arguments, err)
	}

	// Add authentication
	if err := c.addAuthentication(req); err != nil {
		return nil, fmt.Errorf("failed to add authentication to request %s %s (scheme: %s): %w", endpoint.Method, endpoint.Path, c.config.Auth.DefaultScheme, err)
	}

	// Add default headers
	c.addDefaultHeaders(req)

	// Execute with retries
	response, err := c.executeWithRetries(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request execution failed for %s %s (URL: %s, retries: %d): %w", endpoint.Method, endpoint.Path, req.URL.String(), c.config.HTTP.Retries, err)
	}

	c.logger.Debug("Request completed", zap.Int("statusCode", response.StatusCode), zap.String("status", http.StatusText(response.StatusCode)))
	return response, nil
}

// buildRequest builds an HTTP request from endpoint and arguments
func (c *Client) buildRequest(endpoint *types.SwaggerEndpoint, arguments map[string]interface{}) (*http.Request, error) {
	// Start with the endpoint path
	requestPath := endpoint.Path

	// Replace path parameters
	pathParams := make(map[string]string)
	queryParams := url.Values{}
	headers := make(map[string]string)
	var requestBody []byte

	// Process parameters
	for _, param := range endpoint.Parameters {
		argValue, exists := arguments[param.Name]
		if !exists {
			if param.Required {
				return nil, fmt.Errorf("required parameter '%s' (type: %s, location: %s) is missing from arguments: %v", param.Name, getParamType(&param), param.In, arguments)
			}
			continue
		}

		valueStr := fmt.Sprintf("%v", argValue)

		switch param.In {
		case "path":
			pathParams[param.Name] = valueStr
		case "query":
			queryParams.Add(param.Name, valueStr)
		case "header":
			headers[param.Name] = valueStr
		case "cookie":
			// TODO: Implement cookie parameters
			c.logger.Warn("Cookie parameter not yet supported", zap.String("paramName", param.Name))
		}
	}

	// Replace path parameters in the URL
	for paramName, paramValue := range pathParams {
		placeholder := fmt.Sprintf("{%s}", paramName)
		requestPath = strings.ReplaceAll(requestPath, placeholder, paramValue)
	}

	// Handle request body
	if requestBodyArg, exists := arguments["requestBody"]; exists {
		var err error
		requestBody, err = json.Marshal(requestBodyArg)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body (type: %T, value: %v): %w", requestBodyArg, requestBodyArg, err)
		}
		headers["Content-Type"] = "application/json"
	}

	// Build full URL (assume single server for now)
	baseURL := c.getBaseURL()
	if baseURL == "" {
		return nil, fmt.Errorf("no base URL configured - cannot build full URL for endpoint %s %s", endpoint.Method, endpoint.Path)
	}

	fullURL := strings.TrimSuffix(baseURL, "/") + requestPath
	if len(queryParams) > 0 {
		fullURL += "?" + queryParams.Encode()
	}

	// Create request
	var bodyReader *bytes.Reader
	if requestBody != nil {
		bodyReader = bytes.NewReader(requestBody)
	}

	req, err := http.NewRequest(strings.ToUpper(endpoint.Method), fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request (method: %s, URL: %s, body size: %d): %w", endpoint.Method, fullURL, len(requestBody), err)
	}

	// Set headers
	for name, value := range headers {
		req.Header.Set(name, value)
	}

	return req, nil
}

// addAuthentication adds authentication to the request
func (c *Client) addAuthentication(req *http.Request) error {
	if c.config.Auth.APIKey != "" {
		// Add API key authentication
		switch c.config.Auth.DefaultScheme {
		case "bearer":
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.Auth.APIKey))
		case "apikey":
			req.Header.Set("X-API-Key", c.config.Auth.APIKey)
		default:
			// Default to bearer token
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.Auth.APIKey))
		}
	}

	// TODO: Implement other authentication methods (basic auth, oauth, etc.)

	return nil
}

// addDefaultHeaders adds default headers to the request
func (c *Client) addDefaultHeaders(req *http.Request) {
	// Set user agent
	if c.config.HTTP.UserAgent != "" {
		req.Header.Set("User-Agent", c.config.HTTP.UserAgent)
	} else {
		req.Header.Set("User-Agent", "swagger-docs-mcp/1.0.0")
	}

	// Set accept header if not already set
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json, */*")
	}
}

// executeWithRetries executes the request with retry logic
func (c *Client) executeWithRetries(req *http.Request) (*Response, error) {
	var lastErr error
	maxRetries := c.config.HTTP.Retries

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retrying (exponential backoff)
			backoffDuration := time.Duration(attempt*attempt) * time.Second
			c.logger.Debug("Retrying request", zap.Duration("backoffDuration", backoffDuration), zap.Int("attempt", attempt), zap.Int("maxRetries", maxRetries))
			time.Sleep(backoffDuration)
		}

		// Clone the request for retry
		clonedReq := c.cloneRequest(req)

		response, err := c.executeRequest(clonedReq)
		if err != nil {
			lastErr = err
			c.logger.Error("Request attempt failed", zap.Int("attempt", attempt+1), zap.Error(err))
			continue
		}

		// Check if we should retry based on status code
		if c.shouldRetry(response.StatusCode) && attempt < maxRetries {
			lastErr = fmt.Errorf("HTTP %d: %s", response.StatusCode, http.StatusText(response.StatusCode))
			c.logger.Debug("Status code requires retry", zap.Int("statusCode", response.StatusCode))
			continue
		}

		return response, nil
	}

	return nil, fmt.Errorf("request failed after %d attempts (URL: %s, last error: %w)", maxRetries+1, req.URL.String(), lastErr)
}

// executeRequest executes a single HTTP request
func (c *Client) executeRequest(req *http.Request) (*Response, error) {
	c.logger.Debug("Making HTTP request", zap.String("method", req.Method), zap.String("url", req.URL.String()))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed (URL: %s, timeout: %v): %w", req.URL.String(), c.config.HTTP.Timeout, err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body (status: %d %s, content-length: %s): %w", resp.StatusCode, resp.Status, resp.Header.Get("Content-Length"), err)
	}

	// Extract headers
	headers := make(map[string]string)
	for name, values := range resp.Header {
		if len(values) > 0 {
			headers[name] = values[0]
		}
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       body,
	}, nil
}

// cloneRequest creates a copy of an HTTP request for retries
func (c *Client) cloneRequest(req *http.Request) *http.Request {
	cloned := req.Clone(req.Context())

	// Copy body if present
	if req.Body != nil && req.GetBody != nil {
		body, err := req.GetBody()
		if err == nil {
			cloned.Body = body
		}
	}

	return cloned
}

// shouldRetry determines if a request should be retried based on status code
func (c *Client) shouldRetry(statusCode int) bool {
	// Retry on server errors (5xx) and some client errors
	retryableCodes := []int{
		429, // Too Many Requests
		500, // Internal Server Error
		502, // Bad Gateway
		503, // Service Unavailable
		504, // Gateway Timeout
	}

	for _, code := range retryableCodes {
		if statusCode == code {
			return true
		}
	}

	return false
}

// getBaseURL returns the base URL for API requests
func (c *Client) getBaseURL() string {
	// TODO: This should be extracted from swagger documents or configuration
	// For now, return a placeholder that should be configured
	if baseURL := c.config.Auth.DefaultScheme; baseURL != "" {
		// This is a hack - we're reusing the defaultScheme field for base URL
		// In a real implementation, this should be properly configured
		return "https://api.weather.com"
	}

	return "https://api.weather.com" // Default weather API base URL
}

// SetBaseURL sets the base URL for requests (for testing)
func (c *Client) SetBaseURL(baseURL string) {
	// This is a temporary method for testing
	// In production, base URL should come from swagger document servers
}

// GetStatistics returns HTTP client statistics
func (c *Client) GetStatistics() map[string]interface{} {
	return map[string]interface{}{
		"timeout":   c.config.HTTP.Timeout.String(),
		"retries":   c.config.HTTP.Retries,
		"userAgent": c.config.HTTP.UserAgent,
	}
}

// getParamType safely extracts parameter type information
func getParamType(param *types.SwaggerParameter) string {
	if param.Schema == nil {
		return "unknown"
	}

	if schemaMap, ok := param.Schema.(map[string]interface{}); ok {
		if paramType, ok := schemaMap["type"].(string); ok {
			return paramType
		}
	}

	return "unknown"
}
