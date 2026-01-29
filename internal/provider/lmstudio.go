package provider

import "context"

const defaultLMStudioEndpoint = "http://localhost:1234/v1"

// LMStudioProvider implements the Provider interface for LM Studio's local API.
// LM Studio uses an OpenAI-compatible API running locally.
type LMStudioProvider struct {
	*OpenAICompatProvider
}

// NewLMStudioProvider creates a new LM Studio provider.
func NewLMStudioProvider(model, endpoint string) *LMStudioProvider {
	if model == "" {
		model = "local-model"
	}
	if endpoint == "" {
		endpoint = defaultLMStudioEndpoint
	}

	return &LMStudioProvider{
		OpenAICompatProvider: NewOpenAICompatProvider(OpenAICompatConfig{
			Name:      "lmstudio",
			APIKeyEnv: "", // LM Studio typically doesn't require API key
			Endpoint:  endpoint,
			Model:     model,
			MaxTokens: 4096,
		}),
	}
}

// Name returns the provider's identifier.
func (p *LMStudioProvider) Name() string {
	return "lmstudio"
}

// HealthCheck verifies LM Studio is accessible.
func (p *LMStudioProvider) HealthCheck(ctx context.Context) error {
	// LM Studio doesn't require API key, so skip that check
	// Just try to list models
	return p.OpenAICompatProvider.HealthCheck(ctx)
}
