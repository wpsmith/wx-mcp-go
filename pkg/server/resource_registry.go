package server

import (
	"sync"

	"swagger-docs-mcp/pkg/types"
)

// ResourceRegistry manages resources
type ResourceRegistry struct {
	resources map[string]*types.GeneratedResource
	uriIndex  map[string]*types.GeneratedResource
	mutex     sync.RWMutex
}

// NewResourceRegistry creates a new resource registry
func NewResourceRegistry() *ResourceRegistry {
	return &ResourceRegistry{
		resources: make(map[string]*types.GeneratedResource),
		uriIndex:  make(map[string]*types.GeneratedResource),
	}
}

// RegisterResource registers a new resource
func (r *ResourceRegistry) RegisterResource(resource *types.GeneratedResource) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	r.resources[resource.Name] = resource
	r.uriIndex[resource.URI] = resource
	return nil
}

// GetResource retrieves a resource by name
func (r *ResourceRegistry) GetResource(name string) *types.GeneratedResource {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	return r.resources[name]
}

// GetResourceByURI retrieves a resource by URI
func (r *ResourceRegistry) GetResourceByURI(uri string) *types.GeneratedResource {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	return r.uriIndex[uri]
}

// GetAllResources returns all registered resources
func (r *ResourceRegistry) GetAllResources() []*types.GeneratedResource {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	resources := make([]*types.GeneratedResource, 0, len(r.resources))
	for _, resource := range r.resources {
		resources = append(resources, resource)
	}
	
	return resources
}

// GetResourceCount returns the number of registered resources
func (r *ResourceRegistry) GetResourceCount() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	return len(r.resources)
}

// RemoveResource removes a resource by name
func (r *ResourceRegistry) RemoveResource(name string) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	if resource, exists := r.resources[name]; exists {
		delete(r.resources, name)
		delete(r.uriIndex, resource.URI)
		return true
	}
	
	return false
}

// RemoveResourceByURI removes a resource by URI
func (r *ResourceRegistry) RemoveResourceByURI(uri string) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	if resource, exists := r.uriIndex[uri]; exists {
		delete(r.resources, resource.Name)
		delete(r.uriIndex, uri)
		return true
	}
	
	return false
}

// Clear removes all resources
func (r *ResourceRegistry) Clear() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	
	r.resources = make(map[string]*types.GeneratedResource)
	r.uriIndex = make(map[string]*types.GeneratedResource)
}

// HasResource checks if a resource exists by name
func (r *ResourceRegistry) HasResource(name string) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	_, exists := r.resources[name]
	return exists
}

// HasResourceURI checks if a resource exists by URI
func (r *ResourceRegistry) HasResourceURI(uri string) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	_, exists := r.uriIndex[uri]
	return exists
}

// GetResourcesByCategory returns resources filtered by category
func (r *ResourceRegistry) GetResourcesByCategory(category types.ResourceCategory) []*types.GeneratedResource {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	var filtered []*types.GeneratedResource
	for _, resource := range r.resources {
		if resource.Category == category {
			filtered = append(filtered, resource)
		}
	}
	
	return filtered
}

// GetResourcesByMimeType returns resources filtered by MIME type
func (r *ResourceRegistry) GetResourcesByMimeType(mimeType string) []*types.GeneratedResource {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	
	var filtered []*types.GeneratedResource
	for _, resource := range r.resources {
		if resource.MimeType == mimeType {
			filtered = append(filtered, resource)
		}
	}
	
	return filtered
}