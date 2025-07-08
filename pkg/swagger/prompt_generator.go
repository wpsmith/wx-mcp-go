package swagger

import (
	"fmt"
	"regexp"
	"strings"

	"go.uber.org/zap"
	"swagger-docs-mcp/pkg/types"
	"swagger-docs-mcp/pkg/utils"
)

// PromptGenerator generates prompts from Swagger documents
type PromptGenerator struct {
	logger *utils.Logger
	config *types.PromptsConfig
}

// NewPromptGenerator creates a new prompt generator
func NewPromptGenerator(logger *utils.Logger, config *types.PromptsConfig) *PromptGenerator {
	return &PromptGenerator{
		logger: logger.Child("prompt-generator"),
		config: config,
	}
}

// GeneratePromptsFromDocument generates prompts from a parsed Swagger document
func (g *PromptGenerator) GeneratePromptsFromDocument(doc *types.SwaggerDocument, docInfo *types.SwaggerDocumentInfo) ([]*types.GeneratedPrompt, error) {
	if !g.config.Enabled {
		return nil, nil
	}

	// Extract endpoints from the document
	parser := NewParser(g.logger)
	endpoints, err := parser.ExtractEndpoints(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to extract endpoints: %w", err)
	}

	var prompts []*types.GeneratedPrompt
	
	// Generate endpoint-based prompts
	if g.config.GenerateFromEndpoints {
		endpointPrompts, err := g.generateEndpointPrompts(endpoints, docInfo)
		if err != nil {
			g.logger.Error("Failed to generate endpoint prompts", zap.Error(err))
		} else {
			prompts = append(prompts, endpointPrompts...)
		}
	}

	// Generate category-based prompts
	categoryPrompts := g.generateCategoryPrompts(endpoints, docInfo)
	prompts = append(prompts, categoryPrompts...)

	// Generate comparison and analysis prompts
	analysisPrompts := g.generateAnalysisPrompts(endpoints, docInfo)
	prompts = append(prompts, analysisPrompts...)

	g.logger.Debug("Generated prompts from document",
		zap.String("document", docInfo.FilePath),
		zap.Int("promptCount", len(prompts)))

	return prompts, nil
}

// generateEndpointPrompts generates prompts for individual endpoints
func (g *PromptGenerator) generateEndpointPrompts(endpoints []types.SwaggerEndpoint, docInfo *types.SwaggerDocumentInfo) ([]*types.GeneratedPrompt, error) {
	var prompts []*types.GeneratedPrompt

	for _, endpoint := range endpoints {
		// Skip if endpoint doesn't match categories
		if !g.shouldIncludeEndpoint(&endpoint) {
			continue
		}

		prompt := g.createEndpointPrompt(&endpoint, docInfo)
		if prompt != nil {
			prompts = append(prompts, prompt)
		}
	}

	return prompts, nil
}

// generateCategoryPrompts generates category-based prompts
func (g *PromptGenerator) generateCategoryPrompts(endpoints []types.SwaggerEndpoint, docInfo *types.SwaggerDocumentInfo) []*types.GeneratedPrompt {
	var prompts []*types.GeneratedPrompt

	// Group endpoints by category
	categoryEndpoints := make(map[types.WeatherPromptCategory][]*types.SwaggerEndpoint)
	
	for _, endpoint := range endpoints {
		category := g.categorizeEndpoint(&endpoint)
		if category != "" {
			categoryEndpoints[category] = append(categoryEndpoints[category], &endpoint)
		}
	}

	// Generate prompts for each category
	for category, endpoints := range categoryEndpoints {
		if len(endpoints) == 0 {
			continue
		}

		prompt := g.createCategoryPrompt(category, endpoints, docInfo)
		if prompt != nil {
			prompts = append(prompts, prompt)
		}
	}

	return prompts
}

// generateAnalysisPrompts generates analysis and comparison prompts
func (g *PromptGenerator) generateAnalysisPrompts(endpoints []types.SwaggerEndpoint, docInfo *types.SwaggerDocumentInfo) []*types.GeneratedPrompt {
	var prompts []*types.GeneratedPrompt

	// Generate data comparison prompt
	if g.hasMultipleDataTypes(endpoints) {
		prompt := g.createComparisonPrompt(endpoints, docInfo)
		if prompt != nil {
			prompts = append(prompts, prompt)
		}
	}

	// Generate analysis prompt
	analysisPrompt := g.createAnalysisPrompt(endpoints, docInfo)
	if analysisPrompt != nil {
		prompts = append(prompts, analysisPrompt)
	}

	return prompts
}

// createEndpointPrompt creates a prompt for a specific endpoint
func (g *PromptGenerator) createEndpointPrompt(endpoint *types.SwaggerEndpoint, docInfo *types.SwaggerDocumentInfo) *types.GeneratedPrompt {
	category := g.categorizeEndpoint(endpoint)
	if category == "" {
		return nil
	}

	// Create prompt name
	name := g.createPromptName(endpoint.Path, endpoint.Method, "endpoint")
	
	// Create description
	description := fmt.Sprintf("Get %s data", strings.ToLower(endpoint.Summary))
	if endpoint.Description != "" {
		description = endpoint.Description
	}

	// Create template
	template := g.createEndpointTemplate(endpoint, category)
	
	// Create arguments
	arguments := g.createEndpointArguments(endpoint)

	// Create examples
	var examples []types.PromptExample
	if g.config.IncludeExamples {
		examples = g.createEndpointExamples(endpoint)
	}

	return &types.GeneratedPrompt{
		Name:        name,
		Description: description,
		Arguments:   arguments,
		Category:    category,
		Template:    template,
		Examples:    examples,
		Tags:        g.createEndpointTags(endpoint),
		Source:      docInfo,
	}
}

// createCategoryPrompt creates a prompt for a category of endpoints
func (g *PromptGenerator) createCategoryPrompt(category types.WeatherPromptCategory, endpoints []*types.SwaggerEndpoint, docInfo *types.SwaggerDocumentInfo) *types.GeneratedPrompt {
	name := fmt.Sprintf("get-%s-overview", string(category))
	description := fmt.Sprintf("Get comprehensive %s information", string(category))
	
	template := g.createCategoryTemplate(category, endpoints)
	arguments := g.createCategoryArguments(category, endpoints)

	var examples []types.PromptExample
	if g.config.IncludeExamples {
		examples = g.createCategoryExamples(category, endpoints)
	}

	return &types.GeneratedPrompt{
		Name:        name,
		Description: description,
		Arguments:   arguments,
		Category:    category,
		Template:    template,
		Examples:    examples,
		Tags:        []string{string(category), "overview", "comprehensive"},
		Source:      docInfo,
	}
}

// createComparisonPrompt creates a prompt for comparing different data types
func (g *PromptGenerator) createComparisonPrompt(endpoints []types.SwaggerEndpoint, docInfo *types.SwaggerDocumentInfo) *types.GeneratedPrompt {
	return &types.GeneratedPrompt{
		Name:        "compare-weather-data",
		Description: "Compare different weather data sources and formats",
		Category:    types.Comparison,
		Template:    g.createComparisonTemplate(endpoints),
		Arguments: []types.MCPPromptArgument{
			{
				Name:        "location",
				Description: "Location for weather data comparison",
				Required:    true,
			},
			{
				Name:        "data_types",
				Description: "Comma-separated list of data types to compare",
				Required:    false,
			},
		},
		Examples: []types.PromptExample{
			{
				Description: "Compare current conditions from multiple sources",
				Arguments: map[string]interface{}{
					"location":   "New York, NY",
					"data_types": "current,forecast,alerts",
				},
			},
		},
		Tags:   []string{"comparison", "analysis", "multiple-sources"},
		Source: docInfo,
	}
}

// createAnalysisPrompt creates a prompt for analyzing weather data
func (g *PromptGenerator) createAnalysisPrompt(endpoints []types.SwaggerEndpoint, docInfo *types.SwaggerDocumentInfo) *types.GeneratedPrompt {
	return &types.GeneratedPrompt{
		Name:        "analyze-weather-patterns",
		Description: "Analyze weather patterns and trends",
		Category:    types.Analysis,
		Template:    g.createAnalysisTemplate(endpoints),
		Arguments: []types.MCPPromptArgument{
			{
				Name:        "location",
				Description: "Location for weather analysis",
				Required:    true,
			},
			{
				Name:        "time_period",
				Description: "Time period for analysis (e.g., '7 days', '1 month')",
				Required:    false,
			},
			{
				Name:        "focus_areas",
				Description: "Specific areas to focus on (e.g., 'temperature', 'precipitation')",
				Required:    false,
			},
		},
		Examples: []types.PromptExample{
			{
				Description: "Analyze temperature trends over the past week",
				Arguments: map[string]interface{}{
					"location":     "Chicago, IL",
					"time_period":  "7 days",
					"focus_areas":  "temperature,precipitation",
				},
			},
		},
		Tags:   []string{"analysis", "patterns", "trends"},
		Source: docInfo,
	}
}

// Helper methods

// shouldIncludeEndpoint checks if an endpoint should be included based on categories
func (g *PromptGenerator) shouldIncludeEndpoint(endpoint *types.SwaggerEndpoint) bool {
	if len(g.config.Categories) == 0 {
		return true
	}

	category := g.categorizeEndpoint(endpoint)
	for _, allowedCategory := range g.config.Categories {
		if string(category) == allowedCategory {
			return true
		}
	}

	return false
}

// categorizeEndpoint categorizes an endpoint based on its path and description
func (g *PromptGenerator) categorizeEndpoint(endpoint *types.SwaggerEndpoint) types.WeatherPromptCategory {
	path := strings.ToLower(endpoint.Path)
	summary := strings.ToLower(endpoint.Summary)
	description := strings.ToLower(endpoint.Description)
	
	text := fmt.Sprintf("%s %s %s", path, summary, description)

	// Current conditions
	if g.containsAny(text, []string{"current", "conditions", "now", "present"}) {
		return types.CurrentConditions
	}

	// Forecast
	if g.containsAny(text, []string{"forecast", "prediction", "future", "daily", "hourly"}) {
		return types.Forecast
	}

	// Alerts
	if g.containsAny(text, []string{"alert", "warning", "watch", "advisory"}) {
		return types.Alerts
	}

	// Historical
	if g.containsAny(text, []string{"history", "historical", "past", "archive"}) {
		return types.Historical
	}

	// Marine
	if g.containsAny(text, []string{"marine", "ocean", "sea", "wave", "tide"}) {
		return types.Marine
	}

	// Aviation
	if g.containsAny(text, []string{"aviation", "flight", "airport", "metar", "taf"}) {
		return types.Aviation
	}

	// Lifestyle
	if g.containsAny(text, []string{"lifestyle", "index", "comfort", "activity"}) {
		return types.Lifestyle
	}

	return ""
}

// containsAny checks if text contains any of the given keywords
func (g *PromptGenerator) containsAny(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

// hasMultipleDataTypes checks if endpoints have multiple data types
func (g *PromptGenerator) hasMultipleDataTypes(endpoints []types.SwaggerEndpoint) bool {
	categories := make(map[types.WeatherPromptCategory]bool)
	
	for _, endpoint := range endpoints {
		category := g.categorizeEndpoint(&endpoint)
		if category != "" {
			categories[category] = true
		}
	}

	return len(categories) > 1
}

// createPromptName creates a standardized prompt name
func (g *PromptGenerator) createPromptName(path, method, suffix string) string {
	// Clean path for name
	re := regexp.MustCompile(`[^a-zA-Z0-9\-_]`)
	cleanPath := re.ReplaceAllString(path, "-")
	cleanPath = strings.Trim(cleanPath, "-")
	
	// Remove consecutive dashes
	re2 := regexp.MustCompile(`-+`)
	cleanPath = re2.ReplaceAllString(cleanPath, "-")
	
	name := fmt.Sprintf("%s-%s", strings.ToLower(method), cleanPath)
	if suffix != "" {
		name = fmt.Sprintf("%s-%s", name, suffix)
	}
	
	return name
}

// createEndpointTemplate creates a template for an endpoint prompt
func (g *PromptGenerator) createEndpointTemplate(endpoint *types.SwaggerEndpoint, category types.WeatherPromptCategory) string {
	template := fmt.Sprintf("I need to get %s data", strings.ToLower(string(category)))
	
	if endpoint.Description != "" {
		template += fmt.Sprintf(" - specifically: %s", endpoint.Description)
	}
	
	template += "\n\nPlease provide the data in a clear, structured format."
	
	// Add category-specific instructions
	switch category {
	case types.CurrentConditions:
		template += "\n\nInclude current temperature, humidity, wind conditions, and visibility."
	case types.Forecast:
		template += "\n\nInclude forecast periods, expected conditions, and confidence levels."
	case types.Alerts:
		template += "\n\nInclude alert types, severity levels, and affected areas."
	case types.Historical:
		template += "\n\nInclude historical trends and comparisons to normal conditions."
	}
	
	return template
}

// createEndpointArguments creates arguments for an endpoint prompt
func (g *PromptGenerator) createEndpointArguments(endpoint *types.SwaggerEndpoint) []types.MCPPromptArgument {
	var arguments []types.MCPPromptArgument
	
	// Add common location argument
	arguments = append(arguments, types.MCPPromptArgument{
		Name:        "location",
		Description: "Location for weather data (e.g., 'New York, NY' or coordinates)",
		Required:    true,
	})
	
	// Add endpoint-specific arguments based on parameters
	for _, param := range endpoint.Parameters {
		if param.Name == "location" || param.Name == "lat" || param.Name == "lon" {
			continue // Skip location params as we handle them above
		}
		
		arguments = append(arguments, types.MCPPromptArgument{
			Name:        param.Name,
			Description: param.Description,
			Required:    param.Required,
		})
	}
	
	return arguments
}

// createEndpointExamples creates examples for an endpoint prompt
func (g *PromptGenerator) createEndpointExamples(endpoint *types.SwaggerEndpoint) []types.PromptExample {
	var examples []types.PromptExample
	
	// Create a basic example
	example := types.PromptExample{
		Description: fmt.Sprintf("Get %s for New York", strings.ToLower(endpoint.Summary)),
		Arguments: map[string]interface{}{
			"location": "New York, NY",
		},
	}
	
	examples = append(examples, example)
	
	return examples
}

// createEndpointTags creates tags for an endpoint prompt
func (g *PromptGenerator) createEndpointTags(endpoint *types.SwaggerEndpoint) []string {
	var tags []string
	
	// Add method tag
	tags = append(tags, strings.ToLower(endpoint.Method))
	
	// Add category tag
	category := g.categorizeEndpoint(endpoint)
	if category != "" {
		tags = append(tags, string(category))
	}
	
	// Add endpoint tag
	tags = append(tags, "endpoint")
	
	return tags
}

// createCategoryTemplate creates a template for a category prompt
func (g *PromptGenerator) createCategoryTemplate(category types.WeatherPromptCategory, endpoints []*types.SwaggerEndpoint) string {
	template := fmt.Sprintf("I need comprehensive %s information", string(category))
	
	if len(endpoints) > 1 {
		template += fmt.Sprintf(" from %d available data sources", len(endpoints))
	}
	
	template += "\n\nPlease provide:"
	
	// Add category-specific details
	switch category {
	case types.CurrentConditions:
		template += "\n- Current temperature, humidity, and pressure"
		template += "\n- Wind speed and direction"
		template += "\n- Visibility and cloud cover"
		template += "\n- Any significant weather conditions"
	case types.Forecast:
		template += "\n- Multi-day forecast with daily summaries"
		template += "\n- Hourly details for the next 24-48 hours"
		template += "\n- Probability of precipitation"
		template += "\n- Temperature trends and extremes"
	case types.Alerts:
		template += "\n- All active weather alerts and warnings"
		template += "\n- Severity levels and affected areas"
		template += "\n- Timing and expected impacts"
		template += "\n- Recommended actions if applicable"
	}
	
	return template
}

// createCategoryArguments creates arguments for a category prompt
func (g *PromptGenerator) createCategoryArguments(category types.WeatherPromptCategory, endpoints []*types.SwaggerEndpoint) []types.MCPPromptArgument {
	var arguments []types.MCPPromptArgument
	
	// Add common location argument
	arguments = append(arguments, types.MCPPromptArgument{
		Name:        "location",
		Description: "Location for weather data",
		Required:    true,
	})
	
	// Add category-specific arguments
	switch category {
	case types.Forecast:
		arguments = append(arguments, types.MCPPromptArgument{
			Name:        "days",
			Description: "Number of forecast days (default: 5)",
			Required:    false,
		})
	case types.Historical:
		arguments = append(arguments, types.MCPPromptArgument{
			Name:        "start_date",
			Description: "Start date for historical data (YYYY-MM-DD)",
			Required:    false,
		})
		arguments = append(arguments, types.MCPPromptArgument{
			Name:        "end_date",
			Description: "End date for historical data (YYYY-MM-DD)",
			Required:    false,
		})
	}
	
	return arguments
}

// createCategoryExamples creates examples for a category prompt
func (g *PromptGenerator) createCategoryExamples(category types.WeatherPromptCategory, endpoints []*types.SwaggerEndpoint) []types.PromptExample {
	var examples []types.PromptExample
	
	example := types.PromptExample{
		Description: fmt.Sprintf("Get %s overview for Chicago", string(category)),
		Arguments: map[string]interface{}{
			"location": "Chicago, IL",
		},
	}
	
	// Add category-specific example arguments
	switch category {
	case types.Forecast:
		example.Arguments["days"] = 7
	case types.Historical:
		example.Arguments["start_date"] = "2024-01-01"
		example.Arguments["end_date"] = "2024-01-07"
	}
	
	examples = append(examples, example)
	
	return examples
}

// createComparisonTemplate creates a template for comparison prompts
func (g *PromptGenerator) createComparisonTemplate(endpoints []types.SwaggerEndpoint) string {
	return `I need to compare weather data from multiple sources to get a comprehensive view.

Please provide:
- Side-by-side comparison of the requested data types
- Highlight any significant differences between sources
- Explain potential reasons for discrepancies
- Recommend the most reliable source for each data type

Format the comparison in a clear, easy-to-read table or structured format.`
}

// createAnalysisTemplate creates a template for analysis prompts
func (g *PromptGenerator) createAnalysisTemplate(endpoints []types.SwaggerEndpoint) string {
	return `I need a detailed analysis of weather patterns and trends.

Please provide:
- Trend analysis over the specified time period
- Comparison to historical averages or norms
- Identification of notable patterns or anomalies
- Implications for the specified focus areas
- Recommendations or insights based on the analysis

Present the analysis with clear explanations and supporting data.`
}