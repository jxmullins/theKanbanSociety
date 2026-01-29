package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const defaultOllamaURL = "http://localhost:11434"

// OllamaProvider implements the Provider interface for Ollama's REST API.
type OllamaProvider struct {
	*BaseProvider
}

// NewOllamaProvider creates a new Ollama provider.
func NewOllamaProvider(model, endpoint string) *OllamaProvider {
	if model == "" {
		model = "llama3.2"
	}
	if endpoint == "" {
		endpoint = defaultOllamaURL
	}

	return &OllamaProvider{
		BaseProvider: NewBaseProvider(BaseConfig{
			Name:      "ollama",
			BaseURL:   endpoint,
			Model:     model,
			MaxTokens: 4096,
		}),
	}
}

// ollamaRequest represents the request body for the Ollama API.
type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  *ollamaOptions  `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaOptions struct {
	NumPredict  int     `json:"num_predict,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

// ollamaResponse represents the response from the Ollama API.
type ollamaResponse struct {
	Model              string        `json:"model"`
	CreatedAt          string        `json:"created_at"`
	Message            ollamaMessage `json:"message"`
	Done               bool          `json:"done"`
	DoneReason         string        `json:"done_reason"`
	TotalDuration      int64         `json:"total_duration"`
	LoadDuration       int64         `json:"load_duration"`
	PromptEvalCount    int           `json:"prompt_eval_count"`
	PromptEvalDuration int64         `json:"prompt_eval_duration"`
	EvalCount          int           `json:"eval_count"`
	EvalDuration       int64         `json:"eval_duration"`
}

// ollamaStreamResponse represents a streaming response chunk.
type ollamaStreamResponse struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   ollamaMessage `json:"message"`
	Done      bool          `json:"done"`
}

// ollamaErrorResponse represents an error response from the API.
type ollamaErrorResponse struct {
	Error string `json:"error"`
}

// Invoke sends a request to the Ollama API and returns the complete response.
func (p *OllamaProvider) Invoke(ctx context.Context, req Request) (*Response, error) {
	messages := []ollamaMessage{}
	if req.SystemPrompt != "" {
		messages = append(messages, ollamaMessage{Role: "system", Content: req.SystemPrompt})
	}
	messages = append(messages, ollamaMessage{Role: "user", Content: req.Prompt})

	apiReq := ollamaRequest{
		Model:    p.model,
		Messages: messages,
		Stream:   false,
	}

	// Add options if specified
	if req.MaxTokens > 0 || req.Temperature > 0 {
		apiReq.Options = &ollamaOptions{}
		if req.MaxTokens > 0 {
			apiReq.Options.NumPredict = req.MaxTokens
		}
		if req.Temperature > 0 {
			apiReq.Options.Temperature = req.Temperature
		}
	}

	url := p.baseURL + "/api/chat"
	resp, err := p.DoRequest(ctx, http.MethodPost, url, apiReq, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ollamaErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var apiResp ollamaResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &Response{
		Content:      apiResp.Message.Content,
		Model:        apiResp.Model,
		FinishReason: apiResp.DoneReason,
		TokensUsed:   apiResp.PromptEvalCount + apiResp.EvalCount,
	}, nil
}

// Stream sends a request to the Ollama API and returns a channel of response chunks.
func (p *OllamaProvider) Stream(ctx context.Context, req Request) (<-chan StreamChunk, error) {
	messages := []ollamaMessage{}
	if req.SystemPrompt != "" {
		messages = append(messages, ollamaMessage{Role: "system", Content: req.SystemPrompt})
	}
	messages = append(messages, ollamaMessage{Role: "user", Content: req.Prompt})

	apiReq := ollamaRequest{
		Model:    p.model,
		Messages: messages,
		Stream:   true,
	}

	// Add options if specified
	if req.MaxTokens > 0 || req.Temperature > 0 {
		apiReq.Options = &ollamaOptions{}
		if req.MaxTokens > 0 {
			apiReq.Options.NumPredict = req.MaxTokens
		}
		if req.Temperature > 0 {
			apiReq.Options.Temperature = req.Temperature
		}
	}

	url := p.baseURL + "/api/chat"
	resp, err := p.DoRequest(ctx, http.MethodPost, url, apiReq, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var errResp ollamaErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	out := make(chan StreamChunk, 100)

	// Ollama uses newline-delimited JSON, not SSE
	go func() {
		defer resp.Body.Close()
		defer close(out)

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var chunk ollamaStreamResponse
			if err := json.Unmarshal(line, &chunk); err != nil {
				out <- StreamChunk{Error: fmt.Errorf("parsing stream chunk: %w", err)}
				return
			}

			if chunk.Message.Content != "" {
				out <- StreamChunk{Content: chunk.Message.Content}
			}

			if chunk.Done {
				out <- StreamChunk{Done: true}
				return
			}
		}

		if err := scanner.Err(); err != nil {
			out <- StreamChunk{Error: fmt.Errorf("reading stream: %w", err)}
		}
	}()

	return out, nil
}

// HealthCheck verifies the Ollama API is accessible.
func (p *OllamaProvider) HealthCheck(ctx context.Context) error {
	// Check if the server is running by hitting the version endpoint
	url := p.baseURL + "/api/version"
	resp, err := p.DoRequest(ctx, http.MethodGet, url, nil, nil)
	if err != nil {
		return fmt.Errorf("Ollama not accessible at %s: %w", p.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama returned status %d", resp.StatusCode)
	}

	return nil
}

// ListModels returns the list of available models from Ollama.
func (p *OllamaProvider) ListModels(ctx context.Context) ([]string, error) {
	url := p.baseURL + "/api/tags"
	resp, err := p.DoRequest(ctx, http.MethodGet, url, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	models := make([]string, len(result.Models))
	for i, m := range result.Models {
		models[i] = m.Name
	}

	return models, nil
}
