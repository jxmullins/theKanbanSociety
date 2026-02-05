// Package provider defines the interface for AI provider adapters.
package provider

import (
	"context"
	"fmt"

	"github.com/jxmullins/thekanbansociety/internal/config"
)

// StreamChunk represents a piece of streaming response.
type StreamChunk struct {
	Content string
	Done    bool
	Error   error
}

// Request holds the parameters for an AI invocation.
type Request struct {
	Prompt       string
	SystemPrompt string
	MaxTokens    int
	Temperature  float64
}

// MaxPromptLength is the maximum allowed length for prompts to prevent DOS attacks
const MaxPromptLength = 1000000 // 1MB

// Validate checks if the request is valid
func (r *Request) Validate() error {
	if len(r.Prompt) > MaxPromptLength {
		return fmt.Errorf("prompt too long: %d bytes (max: %d)", len(r.Prompt), MaxPromptLength)
	}
	if len(r.SystemPrompt) > MaxPromptLength {
		return fmt.Errorf("system prompt too long: %d bytes (max: %d)", len(r.SystemPrompt), MaxPromptLength)
	}
	if r.Temperature < 0 || r.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2")
	}
	return nil
}

// Response holds the result of an AI invocation.
type Response struct {
	Content      string
	Model        string
	FinishReason string
	TokensUsed   int
}

// Provider defines the interface that all AI provider adapters must implement.
type Provider interface {
	// Name returns the provider's identifier (e.g., "anthropic", "openai").
	Name() string

	// Invoke sends a request and returns the complete response.
	Invoke(ctx context.Context, req Request) (*Response, error)

	// Stream sends a request and returns a channel of response chunks.
	Stream(ctx context.Context, req Request) (<-chan StreamChunk, error)

	// HealthCheck verifies the provider is accessible.
	HealthCheck(ctx context.Context) error
}

// Registry manages available providers.
type Registry struct {
	providers map[string]Provider
	models    map[string]config.ModelConfig
}

// NewRegistry creates a new provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
		models:    make(map[string]config.ModelConfig),
	}
}

// Register adds a provider to the registry.
func (r *Registry) Register(p Provider) {
	r.providers[p.Name()] = p
}

// RegisterModel adds a model configuration to the registry.
func (r *Registry) RegisterModel(aiID string, cfg config.ModelConfig) {
	r.models[aiID] = cfg
}

// RegisterModels adds multiple model configurations from a config.
func (r *Registry) RegisterModels(models map[string]config.ModelConfig) {
	for aiID, cfg := range models {
		r.RegisterModel(aiID, cfg)
	}
}

// Get returns a provider by name.
func (r *Registry) Get(name string) (Provider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// GetForModel returns the provider configured for a specific model ID.
func (r *Registry) GetForModel(aiID string) (Provider, config.ModelConfig, error) {
	modelCfg, ok := r.models[aiID]
	if !ok {
		return nil, config.ModelConfig{}, fmt.Errorf("model %q not found in registry", aiID)
	}

	provider, ok := r.providers[modelCfg.Provider]
	if !ok {
		return nil, config.ModelConfig{}, fmt.Errorf("provider %q not registered for model %q", modelCfg.Provider, aiID)
	}

	return provider, modelCfg, nil
}

// ListProviders returns the names of all registered providers.
func (r *Registry) ListProviders() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// ListModels returns all registered model IDs.
func (r *Registry) ListModels() []string {
	ids := make([]string, 0, len(r.models))
	for id := range r.models {
		ids = append(ids, id)
	}
	return ids
}

// Invoke is a convenience method to invoke a model by AI ID.
func (r *Registry) Invoke(ctx context.Context, aiID string, req Request) (*Response, error) {
	// Try model lookup first
	provider, _, err := r.GetForModel(aiID)
	if err != nil {
		// Fall back to direct provider lookup (for CLI providers)
		var ok bool
		provider, ok = r.Get(aiID)
		if !ok {
			return nil, fmt.Errorf("no provider found for %q", aiID)
		}
	}

	return provider.Invoke(ctx, req)
}

// Stream is a convenience method to stream a model by AI ID.
func (r *Registry) Stream(ctx context.Context, aiID string, req Request) (<-chan StreamChunk, error) {
	// Try model lookup first
	provider, _, err := r.GetForModel(aiID)
	if err != nil {
		// Fall back to direct provider lookup (for CLI providers)
		var ok bool
		provider, ok = r.Get(aiID)
		if !ok {
			return nil, fmt.Errorf("no provider found for %q", aiID)
		}
	}

	return provider.Stream(ctx, req)
}
