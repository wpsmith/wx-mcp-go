package server

import (
	"sync"

	"swagger-docs-mcp/pkg/types"
)

// PromptRegistry manages prompts
type PromptRegistry struct {
	prompts map[string]*types.GeneratedPrompt
	mutex   sync.RWMutex
}

// NewPromptRegistry creates a new prompt registry
func NewPromptRegistry() *PromptRegistry {
	return &PromptRegistry{
		prompts: make(map[string]*types.GeneratedPrompt),
	}
}

// RegisterPrompt registers a new prompt
func (r *PromptRegistry) RegisterPrompt(prompt *types.GeneratedPrompt) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	r.prompts[prompt.Name] = prompt
	return nil
}

// GetPrompt retrieves a prompt by name
func (r *PromptRegistry) GetPrompt(name string) *types.GeneratedPrompt {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	return r.prompts[name]
}

// GetAllPrompts returns all registered prompts
func (r *PromptRegistry) GetAllPrompts() []*types.GeneratedPrompt {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	prompts := make([]*types.GeneratedPrompt, 0, len(r.prompts))
	for _, prompt := range r.prompts {
		prompts = append(prompts, prompt)
	}
	
	return prompts
}

// GetPromptCount returns the number of registered prompts
func (r *PromptRegistry) GetPromptCount() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	return len(r.prompts)
}

// RemovePrompt removes a prompt by name
func (r *PromptRegistry) RemovePrompt(name string) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	if _, exists := r.prompts[name]; exists {
		delete(r.prompts, name)
		return true
	}
	
	return false
}

// Clear removes all prompts
func (r *PromptRegistry) Clear() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	r.prompts = make(map[string]*types.GeneratedPrompt)
}

// HasPrompt checks if a prompt exists
func (r *PromptRegistry) HasPrompt(name string) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	_, exists := r.prompts[name]
	return exists
}

// GetPromptsByCategory returns prompts filtered by category
func (r *PromptRegistry) GetPromptsByCategory(category types.WeatherPromptCategory) []*types.GeneratedPrompt {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	var filtered []*types.GeneratedPrompt
	for _, prompt := range r.prompts {
		if prompt.Category == category {
			filtered = append(filtered, prompt)
		}
	}
	
	return filtered
}