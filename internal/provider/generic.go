package provider

// GenericProvider implements the Provider interface for any OpenAI-compatible API.
// This is a fallback for providers not explicitly supported.
type GenericProvider struct {
	*OpenAICompatProvider
}

// GenericConfig holds configuration for a generic provider.
type GenericConfig struct {
	Name      string
	Endpoint  string
	Model     string
	APIKeyEnv string
	MaxTokens int
}

// NewGenericProvider creates a new generic OpenAI-compatible provider.
func NewGenericProvider(cfg GenericConfig) *GenericProvider {
	name := cfg.Name
	if name == "" {
		name = "generic"
	}

	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	return &GenericProvider{
		OpenAICompatProvider: NewOpenAICompatProvider(OpenAICompatConfig{
			Name:      name,
			APIKeyEnv: cfg.APIKeyEnv,
			Endpoint:  cfg.Endpoint,
			Model:     cfg.Model,
			MaxTokens: maxTokens,
		}),
	}
}

// Name returns the provider's identifier.
func (p *GenericProvider) Name() string {
	return p.name
}
