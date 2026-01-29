package provider

const xaiAPIEndpoint = "https://api.x.ai/v1"

// XAIProvider implements the Provider interface for xAI's Grok API.
// xAI uses an OpenAI-compatible API.
type XAIProvider struct {
	*OpenAICompatProvider
}

// NewXAIProvider creates a new xAI provider.
func NewXAIProvider(model string) *XAIProvider {
	if model == "" {
		model = "grok-4-1-fast-reasoning"
	}

	return &XAIProvider{
		OpenAICompatProvider: NewOpenAICompatProvider(OpenAICompatConfig{
			Name:      "xai",
			APIKeyEnv: "XAI_API_KEY",
			Endpoint:  xaiAPIEndpoint,
			Model:     model,
			MaxTokens: 8192,
		}),
	}
}

// Name returns the provider's identifier.
func (p *XAIProvider) Name() string {
	return "xai"
}
