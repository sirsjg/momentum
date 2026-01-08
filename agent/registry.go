package agent

import (
	"fmt"
	"sync"
)

// AgentFactory is a function that creates an Agent with the given config
type AgentFactory func(Config) Agent

// Registry manages available agent implementations
type Registry struct {
	agents map[string]AgentFactory
	mu     sync.RWMutex
}

// NewRegistry creates a new agent registry with default agents
func NewRegistry() *Registry {
	r := &Registry{
		agents: make(map[string]AgentFactory),
	}

	// Register default agents
	r.Register("claude", func(cfg Config) Agent {
		return NewClaudeCode(cfg)
	})

	return r
}

// Register adds a new agent factory to the registry
func (r *Registry) Register(name string, factory AgentFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[name] = factory
}

// Unregister removes an agent factory from the registry
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.agents, name)
}

// Create instantiates an agent by name
func (r *Registry) Create(name string, config Config) (Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, ok := r.agents[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrAgentNotFound, name)
	}
	return factory(config), nil
}

// Available returns the names of all registered agents
func (r *Registry) Available() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	return names
}

// Has returns true if an agent with the given name is registered
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.agents[name]
	return ok
}

// DefaultRegistry is the global agent registry
var DefaultRegistry = NewRegistry()

// RegisterAgent registers an agent factory in the default registry
func RegisterAgent(name string, factory AgentFactory) {
	DefaultRegistry.Register(name, factory)
}

// CreateAgent creates an agent from the default registry
func CreateAgent(name string, config Config) (Agent, error) {
	return DefaultRegistry.Create(name, config)
}

// AvailableAgents returns the available agents in the default registry
func AvailableAgents() []string {
	return DefaultRegistry.Available()
}
