package provider

const mistralAPIEndpoint = "https://api.mistral.ai/v1"

// MistralProvider implements the Provider interface for Mistral's API.
// Mistral uses an OpenAI-compatible API.
type MistralProvider struct {
	*OpenAICompatProvider
}

// NewMistralProvider creates a new Mistral provider.
func NewMistralProvider(model string) *MistralProvider {
	if model == "" {
		model = "mistral-large-2512"
	}

	return &MistralProvider{
		OpenAICompatProvider: NewOpenAICompatProvider(OpenAICompatConfig{
			Name:      "mistral",
			APIKeyEnv: "MISTRAL_API_KEY",
			Endpoint:  mistralAPIEndpoint,
			Model:     model,
			MaxTokens: 8192,
		}),
	}
}

// Name returns the provider's identifier.
func (p *MistralProvider) Name() string {
	return "mistral"
}
