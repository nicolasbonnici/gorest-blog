package engines

import (
	"fmt"
	"sync"
)

var (
	registry = make(map[string]Engine)
	mu       sync.RWMutex
)

// Register adds an engine to the global registry.
// This is typically called from an engine's init() function for auto-registration.
// If an engine with the same name already exists, it will be replaced.
func Register(engine Engine) {
	mu.Lock()
	defer mu.Unlock()
	registry[engine.Name()] = engine
}

// Get retrieves an engine by name from the registry.
// Returns the engine and true if found, nil and false otherwise.
func Get(name string) (Engine, bool) {
	mu.RLock()
	defer mu.RUnlock()
	engine, ok := registry[name]
	return engine, ok
}

// MustGet retrieves an engine by name and panics if not found.
// Use this when you're certain the engine exists.
func MustGet(name string) Engine {
	engine, ok := Get(name)
	if !ok {
		panic(fmt.Sprintf("engine %s not registered", name))
	}
	return engine
}

// List returns a slice of all registered engine names.
func List() []string {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

// Clear removes all engines from the registry.
// This is primarily useful for testing.
func Clear() {
	mu.Lock()
	defer mu.Unlock()
	registry = make(map[string]Engine)
}
