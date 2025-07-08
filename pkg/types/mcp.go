package types

// MCP protocol types for Model Context Protocol

// MCPRequest represents a generic MCP request
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPResponse represents a generic MCP response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCPNotification represents an MCP notification
type MCPNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPTool represents an MCP tool
type MCPTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// MCPToolCall represents a tool call request
type MCPToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// MCPToolResult represents a tool execution result
type MCPToolResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// MCPContent represents content in MCP
type MCPContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// MCPPrompt represents an MCP prompt
type MCPPrompt struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Arguments   []MCPPromptArgument `json:"arguments,omitempty"`
}

// MCPPromptArgument represents a prompt argument
type MCPPromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required,omitempty"`
}

// MCPResource represents an MCP resource
type MCPResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// MCPResourceTemplate represents an MCP resource template
type MCPResourceTemplate struct {
	URITemplate string `json:"uriTemplate"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// MCPCapabilities represents MCP server capabilities
type MCPCapabilities struct {
	Tools     *MCPToolsCapability     `json:"tools,omitempty"`
	Prompts   *MCPPromptsCapability   `json:"prompts,omitempty"`
	Resources *MCPResourcesCapability `json:"resources,omitempty"`
	Logging   *MCPLoggingCapability   `json:"logging,omitempty"`
}

// MCPToolsCapability represents tools capability
type MCPToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// MCPPromptsCapability represents prompts capability
type MCPPromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// MCPResourcesCapability represents resources capability
type MCPResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// MCPLoggingCapability represents logging capability
type MCPLoggingCapability struct{}

// MCPInitializeParams represents initialization parameters
type MCPInitializeParams struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    MCPCapabilities `json:"capabilities"`
	ClientInfo      MCPClientInfo   `json:"clientInfo"`
}

// MCPClientInfo represents client information
type MCPClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPInitializeResult represents initialization result
type MCPInitializeResult struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    MCPCapabilities `json:"capabilities"`
	ServerInfo      MCPServerInfo   `json:"serverInfo"`
}

// MCPServerInfo represents server information
type MCPServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPListToolsResult represents the result of listing tools
type MCPListToolsResult struct {
	Tools []MCPTool `json:"tools"`
}

// MCPCallToolParams represents parameters for calling a tool
type MCPCallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// MCPCallToolResult represents the result of calling a tool
type MCPCallToolResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// WeatherPromptCategory represents weather prompt categories
type WeatherPromptCategory string

const (
	CurrentConditions WeatherPromptCategory = "current-conditions"
	Forecast          WeatherPromptCategory = "forecast"
	Alerts            WeatherPromptCategory = "alerts"
	Historical        WeatherPromptCategory = "historical"
	Marine            WeatherPromptCategory = "marine"
	Aviation          WeatherPromptCategory = "aviation"
	Lifestyle         WeatherPromptCategory = "lifestyle"
	Analysis          WeatherPromptCategory = "analysis"
	Comparison        WeatherPromptCategory = "comparison"
)

// GeneratedTool represents a tool generated from a swagger endpoint
type GeneratedTool struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	InputSchema  map[string]interface{} `json:"inputSchema"`
	Endpoint     *SwaggerEndpoint       `json:"endpoint"`
	DocumentInfo *SwaggerDocumentInfo   `json:"documentInfo"`
}

// GeneratedPrompt represents a prompt generated from Swagger documentation
type GeneratedPrompt struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Arguments   []MCPPromptArgument      `json:"arguments,omitempty"`
	Category    WeatherPromptCategory    `json:"category,omitempty"`
	Template    string                   `json:"template"`
	Examples    []PromptExample          `json:"examples,omitempty"`
	Tags        []string                 `json:"tags,omitempty"`
	Source      *SwaggerDocumentInfo     `json:"source,omitempty"`
}

// PromptExample represents a prompt usage example
type PromptExample struct {
	Description string                 `json:"description"`
	Arguments   map[string]interface{} `json:"arguments"`
	ExpectedOutput string              `json:"expectedOutput,omitempty"`
}

// GeneratedResource represents a resource generated from Swagger documentation
type GeneratedResource struct {
	URI         string               `json:"uri"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	MimeType    string               `json:"mimeType"`
	Category    ResourceCategory     `json:"category,omitempty"`
	Tags        []string             `json:"tags,omitempty"`
	Source      *SwaggerDocumentInfo `json:"source,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ResourceCategory represents different categories of resources
type ResourceCategory string

const (
	ResourceCategoryDocumentation ResourceCategory = "documentation"
	ResourceCategorySchema       ResourceCategory = "schema"
	ResourceCategoryExample      ResourceCategory = "example"
	ResourceCategoryReference    ResourceCategory = "reference"
	ResourceCategoryEndpoint     ResourceCategory = "endpoint"
)

// MCPPromptGetParams represents parameters for getting a prompt
type MCPPromptGetParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// MCPPromptGetResult represents the result of getting a prompt
type MCPPromptGetResult struct {
	Description string             `json:"description"`
	Messages    []MCPPromptMessage `json:"messages"`
}

// MCPPromptMessage represents a message in a prompt response
type MCPPromptMessage struct {
	Role    string           `json:"role"`
	Content MCPPromptContent `json:"content"`
}

// MCPPromptContent represents content in a prompt message
type MCPPromptContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MCPListPromptsResult represents the result of listing prompts
type MCPListPromptsResult struct {
	Prompts []MCPPrompt `json:"prompts"`
}

// MCPListResourcesResult represents the result of listing resources
type MCPListResourcesResult struct {
	Resources []MCPResource `json:"resources"`
}

// MCPReadResourceParams represents parameters for reading a resource
type MCPReadResourceParams struct {
	URI string `json:"uri"`
}

// MCPReadResourceResult represents the result of reading a resource
type MCPReadResourceResult struct {
	Contents []MCPResourceContent `json:"contents"`
}

// MCPResourceContent represents the content of a resource
type MCPResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"` // base64 encoded
}
