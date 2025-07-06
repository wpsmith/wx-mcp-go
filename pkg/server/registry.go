package server

import (
	"fmt"
	"sync"

	"swagger-docs-mcp/pkg/types"
)

// ToolRegistry manages the collection of available tools
type ToolRegistry struct {
	tools map[string]*types.GeneratedTool
	mutex sync.RWMutex
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]*types.GeneratedTool),
	}
}

// RegisterTool registers a new tool in the registry
func (r *ToolRegistry) RegisterTool(tool *types.GeneratedTool) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if tool.Name == "" {
		return fmt.Errorf("tool name cannot be empty (endpoint: %s %s, document: %s)",
			tool.Endpoint.Method, tool.Endpoint.Path, tool.DocumentInfo.Title)
	}

	if existing, exists := r.tools[tool.Name]; exists {
		return fmt.Errorf("tool with name '%s' already exists - conflict between:\n  New: %s %s (from %s)\n  Existing: %s %s (from %s)",
			tool.Name,
			tool.Endpoint.Method, tool.Endpoint.Path, tool.DocumentInfo.Title,
			existing.Endpoint.Method, existing.Endpoint.Path, existing.DocumentInfo.Title)
	}

	r.tools[tool.Name] = tool
	return nil
}

// GetTool retrieves a tool by name
func (r *ToolRegistry) GetTool(name string) *types.GeneratedTool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.tools[name]
}

// GetAllTools returns all registered tools
func (r *ToolRegistry) GetAllTools() []*types.GeneratedTool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	tools := make([]*types.GeneratedTool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}

	return tools
}

// GetToolNames returns all tool names
func (r *ToolRegistry) GetToolNames() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}

	return names
}

// GetToolCount returns the number of registered tools
func (r *ToolRegistry) GetToolCount() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return len(r.tools)
}

// HasTool checks if a tool with the given name exists
func (r *ToolRegistry) HasTool(name string) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	_, exists := r.tools[name]
	return exists
}

// UnregisterTool removes a tool from the registry
func (r *ToolRegistry) UnregisterTool(name string) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.tools[name]; exists {
		delete(r.tools, name)
		return true
	}

	return false
}

// Clear removes all tools from the registry
func (r *ToolRegistry) Clear() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.tools = make(map[string]*types.GeneratedTool)
}

// GetToolsByVersion returns tools filtered by API version
func (r *ToolRegistry) GetToolsByVersion(version string) []*types.GeneratedTool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var filtered []*types.GeneratedTool
	for _, tool := range r.tools {
		if tool.DocumentInfo != nil && tool.DocumentInfo.Version == version {
			filtered = append(filtered, tool)
		}
	}

	return filtered
}

// GetToolsByPackageIDs returns tools filtered by package IDs
func (r *ToolRegistry) GetToolsByPackageIDs(packageIDs []string) []*types.GeneratedTool {
	if len(packageIDs) == 0 {
		return r.GetAllTools()
	}

	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var filtered []*types.GeneratedTool
	for _, tool := range r.tools {
		if tool.DocumentInfo != nil && len(tool.DocumentInfo.PackageIDs) > 0 {
			// Check if any of the tool's package IDs match any of the filter IDs
			hasMatch := false
			for _, toolPackageID := range tool.DocumentInfo.PackageIDs {
				for _, filterPackageID := range packageIDs {
					if toolPackageID == filterPackageID {
						hasMatch = true
						break
					}
				}
				if hasMatch {
					break
				}
			}
			if hasMatch {
				filtered = append(filtered, tool)
			}
		}
	}

	return filtered
}

// GetStatistics returns registry statistics
func (r *ToolRegistry) GetStatistics() map[string]interface{} {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	stats := map[string]interface{}{
		"totalTools": len(r.tools),
	}

	// Count tools by version
	versionCounts := make(map[string]int)
	for _, tool := range r.tools {
		if tool.DocumentInfo != nil {
			version := tool.DocumentInfo.Version
			versionCounts[version]++
		}
	}
	stats["toolsByVersion"] = versionCounts

	// Count tools by document
	documentCounts := make(map[string]int)
	for _, tool := range r.tools {
		if tool.DocumentInfo != nil {
			documentPath := tool.DocumentInfo.FilePath
			documentCounts[documentPath]++
		}
	}
	stats["toolsByDocument"] = documentCounts

	return stats
}
