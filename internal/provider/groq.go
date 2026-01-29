package provider

const groqAPIEndpoint = "https://api.groq.com/openai/v1"

// GroqProvider implements the Provider interface for Groq's API.
// Groq uses an OpenAI-compatible API for Llama and other models.
type GroqProvider struct {
	*OpenAICompatProvider
}

// NewGroqProvider creates a new Groq provider.
func NewGroqProvider(model string) *GroqProvider {
	if model == "" {
		model = "llama-4-maverick"
	}

	return &GroqProvider{
		OpenAICompatProvider: NewOpenAICompatProvider(OpenAICompatConfig{
			Name:      "groq",
			APIKeyEnv: "GROQ_API_KEY",
			Endpoint:  groqAPIEndpoint,
			Model:     model,
			MaxTokens: 8192,
		}),
	}
}

// Name returns the provider's identifier.
func (p *GroqProvider) Name() string {
	return "groq"
}
