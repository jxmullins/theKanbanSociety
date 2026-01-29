package provider

const deepseekAPIEndpoint = "https://api.deepseek.com/v1"

// DeepSeekProvider implements the Provider interface for DeepSeek's API.
// DeepSeek uses an OpenAI-compatible API.
type DeepSeekProvider struct {
	*OpenAICompatProvider
}

// NewDeepSeekProvider creates a new DeepSeek provider.
func NewDeepSeekProvider(model string) *DeepSeekProvider {
	if model == "" {
		model = "deepseek-chat"
	}

	return &DeepSeekProvider{
		OpenAICompatProvider: NewOpenAICompatProvider(OpenAICompatConfig{
			Name:      "deepseek",
			APIKeyEnv: "DEEPSEEK_API_KEY",
			Endpoint:  deepseekAPIEndpoint,
			Model:     model,
			MaxTokens: 8192,
		}),
	}
}

// Name returns the provider's identifier.
func (p *DeepSeekProvider) Name() string {
	return "deepseek"
}
