// Package config provides a stage registry and human-readable pipeline configuration.
package config

import (
	"context"
	"fmt"
	"sync"

	"github.com/dcshock/runpipe/pipeline"
)

// Registry maps stage names to pipeline stages. Safe for concurrent use.
type Registry struct {
	mu     sync.RWMutex
	stages map[string]pipeline.Stage
}

// NewRegistry returns an empty stage registry.
func NewRegistry() *Registry {
	return &Registry{stages: make(map[string]pipeline.Stage)}
}

// Register adds a stage under the given name. Overwrites any existing registration.
func (r *Registry) Register(name string, stage pipeline.Stage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stages == nil {
		r.stages = make(map[string]pipeline.Stage)
	}
	r.stages[name] = stage
}

// Get returns the stage for name, or nil and false if not found.
func (r *Registry) Get(name string) (pipeline.Stage, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.stages[name]
	return s, ok
}

// MustGet returns the stage for name, or panics if not found.
func (r *Registry) MustGet(name string) pipeline.Stage {
	s, ok := r.Get(name)
	if !ok {
		panic(fmt.Sprintf("config: stage %q not registered", name))
	}
	return s
}

// Names returns all registered stage names (unordered).
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.stages))
	for n := range r.stages {
		names = append(names, n)
	}
	return names
}

// SourceRegistry maps source names to pipeline sources. Safe for concurrent use.
// Use with PipelineConfig.Source and BuildOptions.SourceRegistry so config can reference a source by name.
type SourceRegistry struct {
	mu      sync.RWMutex
	sources map[string]func(context.Context) (interface{}, error)
}

// NewSourceRegistry returns an empty source registry.
func NewSourceRegistry() *SourceRegistry {
	return &SourceRegistry{sources: make(map[string]func(context.Context) (interface{}, error))}
}

// Register adds a source under the given name. Overwrites any existing registration.
func (r *SourceRegistry) Register(name string, source func(context.Context) (interface{}, error)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sources == nil {
		r.sources = make(map[string]func(context.Context) (interface{}, error))
	}
	r.sources[name] = source
}

// Get returns the source for name, or nil and false if not found.
func (r *SourceRegistry) Get(name string) (func(context.Context) (interface{}, error), bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sources[name]
	return s, ok
}
