package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	anthropicAPIURL     = "https://api.anthropic.com/v1/messages"
	anthropicAPIVersion = "2023-06-01"
)

// AnthropicProvider implements the Provider interface for Anthropic's Claude API.
type AnthropicProvider struct {
	*BaseProvider
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(model string) *AnthropicProvider {
	if model == "" {
		model = "claude-sonnet-4-5-20250929"
	}

	return &AnthropicProvider{
		BaseProvider: NewBaseProvider(BaseConfig{
			Name:      "anthropic",
			APIKeyEnv: "ANTHROPIC_API_KEY",
			BaseURL:   anthropicAPIURL,
			Model:     model,
			MaxTokens: 4096,
		}),
	}
}

// anthropicRequest represents the request body for the Anthropic API.
type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Stream    bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse represents the response from the Anthropic API.
type anthropicResponse struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Role         string `json:"role"`
	Content      []anthropicContent `json:"content"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// anthropicStreamEvent represents a streaming event from the Anthropic API.
type anthropicStreamEvent struct {
	Type         string `json:"type"`
	Index        int    `json:"index"`
	ContentBlock *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content_block"`
	Delta *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
	Message *anthropicResponse `json:"message"`
}

// anthropicErrorResponse represents an error response from the API.
type anthropicErrorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Invoke sends a request to the Anthropic API and returns the complete response.
func (p *AnthropicProvider) Invoke(ctx context.Context, req Request) (*Response, error) {
	if err := p.CheckAPIKeyRequired(); err != nil {
		return nil, err
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = p.maxTokens
	}

	apiReq := anthropicRequest{
		Model:     p.model,
		MaxTokens: maxTokens,
		System:    req.SystemPrompt,
		Messages: []anthropicMessage{
			{Role: "user", Content: req.Prompt},
		},
	}

	headers := map[string]string{
		"x-api-key":         p.GetAPIKey(),
		"anthropic-version": anthropicAPIVersion,
	}

	resp, err := p.DoRequest(ctx, http.MethodPost, anthropicAPIURL, apiReq, headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp anthropicErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	// Extract text content
	var content string
	for _, c := range apiResp.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	return &Response{
		Content:      content,
		Model:        apiResp.Model,
		FinishReason: apiResp.StopReason,
		TokensUsed:   apiResp.Usage.InputTokens + apiResp.Usage.OutputTokens,
	}, nil
}

// Stream sends a request to the Anthropic API and returns a channel of response chunks.
func (p *AnthropicProvider) Stream(ctx context.Context, req Request) (<-chan StreamChunk, error) {
	if err := p.CheckAPIKeyRequired(); err != nil {
		return nil, err
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = p.maxTokens
	}

	apiReq := anthropicRequest{
		Model:     p.model,
		MaxTokens: maxTokens,
		System:    req.SystemPrompt,
		Messages: []anthropicMessage{
			{Role: "user", Content: req.Prompt},
		},
		Stream: true,
	}

	headers := map[string]string{
		"x-api-key":         p.GetAPIKey(),
		"anthropic-version": anthropicAPIVersion,
	}

	resp, err := p.DoRequest(ctx, http.MethodPost, anthropicAPIURL, apiReq, headers)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var errResp anthropicErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	out := make(chan StreamChunk, 100)

	go p.ReadSSEStream(resp, out, func(data []byte) (string, bool, error) {
		var event anthropicStreamEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return "", false, fmt.Errorf("parsing stream event: %w", err)
		}

		switch event.Type {
		case "content_block_delta":
			if event.Delta != nil && event.Delta.Type == "text_delta" {
				return event.Delta.Text, false, nil
			}
		case "message_stop":
			return "", true, nil
		}

		return "", false, nil
	})

	return out, nil
}

// HealthCheck verifies the Anthropic API is accessible.
func (p *AnthropicProvider) HealthCheck(ctx context.Context) error {
	if err := p.CheckAPIKeyRequired(); err != nil {
		return err
	}

	// Send a minimal request to verify connectivity
	_, err := p.Invoke(ctx, Request{
		Prompt:    "Hi",
		MaxTokens: 10,
	})
	return err
}
