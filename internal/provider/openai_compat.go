package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// OpenAICompatProvider is a base for providers that use OpenAI-compatible APIs.
// This includes Groq, DeepSeek, Mistral, xAI, LM Studio, and generic endpoints.
type OpenAICompatProvider struct {
	*BaseProvider
	endpoint string
}

// OpenAICompatConfig holds configuration for creating an OpenAI-compatible provider.
type OpenAICompatConfig struct {
	Name      string
	APIKeyEnv string
	Endpoint  string
	Model     string
	MaxTokens int
}

// NewOpenAICompatProvider creates a new OpenAI-compatible provider.
func NewOpenAICompatProvider(cfg OpenAICompatConfig) *OpenAICompatProvider {
	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	return &OpenAICompatProvider{
		BaseProvider: NewBaseProvider(BaseConfig{
			Name:      cfg.Name,
			APIKeyEnv: cfg.APIKeyEnv,
			BaseURL:   cfg.Endpoint,
			Model:     cfg.Model,
			MaxTokens: maxTokens,
		}),
		endpoint: cfg.Endpoint,
	}
}

// Invoke sends a request to the OpenAI-compatible API and returns the complete response.
func (p *OpenAICompatProvider) Invoke(ctx context.Context, req Request) (*Response, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	if err := p.CheckAPIKeyRequired(); err != nil {
		return nil, err
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = p.maxTokens
	}

	messages := []openaiMessage{}
	if req.SystemPrompt != "" {
		messages = append(messages, openaiMessage{Role: "system", Content: req.SystemPrompt})
	}
	messages = append(messages, openaiMessage{Role: "user", Content: req.Prompt})

	apiReq := openaiRequest{
		Model:     p.model,
		Messages:  messages,
		MaxTokens: maxTokens,
	}

	if req.Temperature > 0 {
		apiReq.Temperature = req.Temperature
	}

	url := p.endpoint + "/chat/completions"
	headers := map[string]string{}
	if apiKey := p.GetAPIKey(); apiKey != "" {
		headers["Authorization"] = "Bearer " + apiKey
	}

	resp, err := p.DoRequest(ctx, http.MethodPost, url, apiReq, headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp openaiErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var apiResp openaiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &Response{
		Content:      apiResp.Choices[0].Message.Content,
		Model:        apiResp.Model,
		FinishReason: apiResp.Choices[0].FinishReason,
		TokensUsed:   apiResp.Usage.TotalTokens,
	}, nil
}

// Stream sends a request to the OpenAI-compatible API and returns a channel of response chunks.
func (p *OpenAICompatProvider) Stream(ctx context.Context, req Request) (<-chan StreamChunk, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	if err := p.CheckAPIKeyRequired(); err != nil {
		return nil, err
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = p.maxTokens
	}

	messages := []openaiMessage{}
	if req.SystemPrompt != "" {
		messages = append(messages, openaiMessage{Role: "system", Content: req.SystemPrompt})
	}
	messages = append(messages, openaiMessage{Role: "user", Content: req.Prompt})

	apiReq := openaiRequest{
		Model:     p.model,
		Messages:  messages,
		MaxTokens: maxTokens,
		Stream:    true,
	}

	if req.Temperature > 0 {
		apiReq.Temperature = req.Temperature
	}

	url := p.endpoint + "/chat/completions"
	headers := map[string]string{}
	if apiKey := p.GetAPIKey(); apiKey != "" {
		headers["Authorization"] = "Bearer " + apiKey
	}

	resp, err := p.DoRequest(ctx, http.MethodPost, url, apiReq, headers)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var errResp openaiErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	out := make(chan StreamChunk, 100)

	go p.ReadSSEStream(resp, out, func(data []byte) (string, bool, error) {
		var chunk openaiStreamChunk
		if err := json.Unmarshal(data, &chunk); err != nil {
			return "", false, fmt.Errorf("parsing stream chunk: %w", err)
		}

		if len(chunk.Choices) == 0 {
			return "", false, nil
		}

		choice := chunk.Choices[0]
		if choice.FinishReason != "" {
			return "", true, nil
		}

		return choice.Delta.Content, false, nil
	})

	return out, nil
}

// HealthCheck verifies the API is accessible.
func (p *OpenAICompatProvider) HealthCheck(ctx context.Context) error {
	if err := p.CheckAPIKeyRequired(); err != nil {
		return err
	}

	// Try to list models
	url := p.endpoint + "/models"
	headers := map[string]string{}
	if apiKey := p.GetAPIKey(); apiKey != "" {
		headers["Authorization"] = "Bearer " + apiKey
	}

	resp, err := p.DoRequest(ctx, http.MethodGet, url, nil, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	return nil
}

// SetEndpoint allows overriding the endpoint.
func (p *OpenAICompatProvider) SetEndpoint(endpoint string) {
	p.endpoint = endpoint
}
