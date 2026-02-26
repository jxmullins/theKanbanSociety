package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const googleAPIBaseURL = "https://generativelanguage.googleapis.com/v1beta/models"

// GoogleProvider implements the Provider interface for Google's Generative AI API (Gemini).
type GoogleProvider struct {
	*BaseProvider
}

// NewGoogleProvider creates a new Google provider.
func NewGoogleProvider(model string) *GoogleProvider {
	if model == "" {
		model = "gemini-3-pro-preview"
	}

	return &GoogleProvider{
		BaseProvider: NewBaseProvider(BaseConfig{
			Name:      "google",
			APIKeyEnv: "GOOGLE_API_KEY",
			BaseURL:   googleAPIBaseURL,
			Model:     model,
			MaxTokens: 8192,
		}),
	}
}

// googleRequest represents the request body for the Google Generative AI API.
type googleRequest struct {
	Contents         []googleContent        `json:"contents"`
	SystemInstruction *googleContent        `json:"systemInstruction,omitempty"`
	GenerationConfig *googleGenerationConfig `json:"generationConfig,omitempty"`
}

type googleContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []googlePart `json:"parts"`
}

type googlePart struct {
	Text string `json:"text"`
}

type googleGenerationConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
}

// googleResponse represents the response from the Google Generative AI API.
type googleResponse struct {
	Candidates []struct {
		Content struct {
			Parts []googlePart `json:"parts"`
			Role  string       `json:"role"`
		} `json:"content"`
		FinishReason  string `json:"finishReason"`
		SafetyRatings []struct {
			Category    string `json:"category"`
			Probability string `json:"probability"`
		} `json:"safetyRatings"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

// googleStreamResponse represents a streaming response chunk from the Google API.
type googleStreamResponse struct {
	Candidates []struct {
		Content struct {
			Parts []googlePart `json:"parts"`
			Role  string       `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason,omitempty"`
	} `json:"candidates"`
}

// googleErrorResponse represents an error response from the API.
type googleErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// Invoke sends a request to the Google Generative AI API and returns the complete response.
func (p *GoogleProvider) Invoke(ctx context.Context, req Request) (*Response, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	if err := p.CheckAPIKeyRequired(); err != nil {
		return nil, err
	}

	apiReq := p.buildRequest(req)
	url := fmt.Sprintf("%s/%s:generateContent?key=%s", googleAPIBaseURL, p.model, p.GetAPIKey())

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
		var errResp googleErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var apiResp googleResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if len(apiResp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	// Extract text content
	var content strings.Builder
	for _, part := range apiResp.Candidates[0].Content.Parts {
		content.WriteString(part.Text)
	}

	return &Response{
		Content:      content.String(),
		Model:        p.model,
		FinishReason: apiResp.Candidates[0].FinishReason,
		TokensUsed:   apiResp.UsageMetadata.TotalTokenCount,
	}, nil
}

// Stream sends a request to the Google Generative AI API and returns a channel of response chunks.
func (p *GoogleProvider) Stream(ctx context.Context, req Request) (<-chan StreamChunk, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	if err := p.CheckAPIKeyRequired(); err != nil {
		return nil, err
	}

	apiReq := p.buildRequest(req)
	url := fmt.Sprintf("%s/%s:streamGenerateContent?alt=sse&key=%s", googleAPIBaseURL, p.model, p.GetAPIKey())

	resp, err := p.DoRequest(ctx, http.MethodPost, url, apiReq, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var errResp googleErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	out := make(chan StreamChunk, 100)

	go func() {
		defer resp.Body.Close()
		defer close(out)

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines and non-data lines
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "" {
				continue
			}

			var chunk googleStreamResponse
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				out <- StreamChunk{Error: fmt.Errorf("parsing stream chunk: %w", err)}
				return
			}

			if len(chunk.Candidates) > 0 && len(chunk.Candidates[0].Content.Parts) > 0 {
				for _, part := range chunk.Candidates[0].Content.Parts {
					if part.Text != "" {
						out <- StreamChunk{Content: part.Text}
					}
				}

				if chunk.Candidates[0].FinishReason != "" {
					out <- StreamChunk{Done: true}
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			out <- StreamChunk{Error: fmt.Errorf("reading stream: %w", err)}
		}
	}()

	return out, nil
}

// buildRequest constructs a Google API request from a provider.Request.
func (p *GoogleProvider) buildRequest(req Request) googleRequest {
	apiReq := googleRequest{
		Contents: []googleContent{
			{
				Role:  "user",
				Parts: []googlePart{{Text: req.Prompt}},
			},
		},
	}

	if req.SystemPrompt != "" {
		apiReq.SystemInstruction = &googleContent{
			Parts: []googlePart{{Text: req.SystemPrompt}},
		}
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = p.maxTokens
	}

	apiReq.GenerationConfig = &googleGenerationConfig{
		MaxOutputTokens: maxTokens,
	}

	if req.Temperature > 0 {
		apiReq.GenerationConfig.Temperature = req.Temperature
	}

	return apiReq
}

// HealthCheck verifies the Google API is accessible.
func (p *GoogleProvider) HealthCheck(ctx context.Context) error {
	if err := p.CheckAPIKeyRequired(); err != nil {
		return err
	}

	// List models to verify connectivity
	url := fmt.Sprintf("%s?key=%s", googleAPIBaseURL, p.GetAPIKey())
	resp, err := p.DoRequest(ctx, http.MethodGet, url, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	return nil
}
