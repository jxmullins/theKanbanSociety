package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const openaiAPIURL = "https://api.openai.com/v1/chat/completions"

// OpenAIProvider implements the Provider interface for OpenAI's Chat Completions API.
type OpenAIProvider struct {
	*BaseProvider
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(model string) *OpenAIProvider {
	if model == "" {
		model = "gpt-5.2"
	}

	return &OpenAIProvider{
		BaseProvider: NewBaseProvider(BaseConfig{
			Name:      "openai",
			APIKeyEnv: "OPENAI_API_KEY",
			BaseURL:   openaiAPIURL,
			Model:     model,
			MaxTokens: 4096,
		}),
	}
}

// openaiRequest represents the request body for the OpenAI API.
type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openaiResponse represents the response from the OpenAI API.
type openaiResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int           `json:"index"`
		Message      openaiMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// openaiStreamChunk represents a streaming chunk from the OpenAI API.
type openaiStreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// openaiErrorResponse represents an error response from the API.
type openaiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// Invoke sends a request to the OpenAI API and returns the complete response.
func (p *OpenAIProvider) Invoke(ctx context.Context, req Request) (*Response, error) {
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

	headers := map[string]string{
		"Authorization": "Bearer " + p.GetAPIKey(),
	}

	resp, err := p.DoRequest(ctx, http.MethodPost, openaiAPIURL, apiReq, headers)
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

// Stream sends a request to the OpenAI API and returns a channel of response chunks.
func (p *OpenAIProvider) Stream(ctx context.Context, req Request) (<-chan StreamChunk, error) {
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

	headers := map[string]string{
		"Authorization": "Bearer " + p.GetAPIKey(),
	}

	resp, err := p.DoRequest(ctx, http.MethodPost, openaiAPIURL, apiReq, headers)
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

// HealthCheck verifies the OpenAI API is accessible.
func (p *OpenAIProvider) HealthCheck(ctx context.Context) error {
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
