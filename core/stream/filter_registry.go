package stream

import (
	"strings"
	"sync"
)

// FilterRegistry holds registered user stream filter class names
type FilterRegistry struct {
	mu       sync.RWMutex
	filters  map[string]string // filter name -> class name
}

var globalFilterRegistry = &FilterRegistry{
	filters: make(map[string]string),
}

// GetFilterRegistry returns the global filter registry
func GetFilterRegistry() *FilterRegistry {
	return globalFilterRegistry
}

// Register registers a user filter class name for a given filter name.
// Returns false if the filter name is already registered (built-in or user).
func (r *FilterRegistry) Register(filterName, className string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if it's a built-in filter name
	if IsBuiltinFilter(filterName) {
		return false
	}

	// Check if the filter name starts with a built-in prefix
	for _, prefix := range []string{"string.", "convert.", "zlib.", "bzip2.", "convert.iconv."} {
		if strings.HasPrefix(filterName, prefix) && IsBuiltinFilter(filterName) {
			return false
		}
	}

	// Check if already registered
	if _, exists := r.filters[filterName]; exists {
		return false
	}

	r.filters[filterName] = className
	return true
}

// Lookup returns the class name for a registered user filter, or "" if not found.
func (r *FilterRegistry) Lookup(filterName string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	className, ok := r.filters[filterName]
	return className, ok
}

// GetAll returns all registered filter names (user-defined)
func (r *FilterRegistry) GetAll() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, 0, len(r.filters))
	for name := range r.filters {
		result = append(result, name)
	}
	return result
}

// Reset clears all registered user filters (useful for testing)
func (r *FilterRegistry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.filters = make(map[string]string)
}
