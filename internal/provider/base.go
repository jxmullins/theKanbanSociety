package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// BaseProvider provides common functionality for HTTP-based providers.
type BaseProvider struct {
	name       string
	client     *http.Client
	apiKey     string
	apiKeyEnv  string
	baseURL    string
	model      string
	maxTokens  int
	timeout    time.Duration
}

// BaseConfig holds configuration for creating a BaseProvider.
type BaseConfig struct {
	Name      string
	APIKeyEnv string
	BaseURL   string
	Model     string
	MaxTokens int
	Timeout   time.Duration
}

// NewBaseProvider creates a new base provider with common configuration.
func NewBaseProvider(cfg BaseConfig) *BaseProvider {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 180 * time.Second
	}

	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	apiKey := ""
	if cfg.APIKeyEnv != "" {
		apiKey = os.Getenv(cfg.APIKeyEnv)
	}

	return &BaseProvider{
		name:      cfg.Name,
		apiKey:    apiKey,
		apiKeyEnv: cfg.APIKeyEnv,
		baseURL:   cfg.BaseURL,
		model:     cfg.Model,
		maxTokens: maxTokens,
		timeout:   timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Name returns the provider's identifier.
func (b *BaseProvider) Name() string {
	return b.name
}

// GetAPIKey returns the API key, loading from environment if needed.
func (b *BaseProvider) GetAPIKey() string {
	if b.apiKey == "" && b.apiKeyEnv != "" {
		b.apiKey = os.Getenv(b.apiKeyEnv)
	}
	return b.apiKey
}

// SetModel allows overriding the model.
func (b *BaseProvider) SetModel(model string) {
	b.model = model
}

// GetModel returns the current model.
func (b *BaseProvider) GetModel() string {
	return b.model
}

// DoRequest performs an HTTP request with common error handling.
func (b *BaseProvider) DoRequest(ctx context.Context, method, url string, body interface{}, headers map[string]string) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}

	return resp, nil
}

// ReadSSEStream reads a Server-Sent Events stream and sends chunks to the channel.
// The parseFunc extracts content from each SSE data line.
func (b *BaseProvider) ReadSSEStream(resp *http.Response, out chan<- StreamChunk, parseFunc func([]byte) (string, bool, error)) {
	defer resp.Body.Close()
	defer close(out)

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()

		// Skip empty lines and comments
		if len(line) == 0 || line[0] == ':' {
			continue
		}

		// Check for data prefix
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}

		data := line[6:] // Remove "data: " prefix

		// Check for stream end marker
		if bytes.Equal(data, []byte("[DONE]")) {
			out <- StreamChunk{Done: true}
			return
		}

		content, done, err := parseFunc(data)
		if err != nil {
			out <- StreamChunk{Error: err}
			return
		}

		if content != "" {
			out <- StreamChunk{Content: content}
		}

		if done {
			out <- StreamChunk{Done: true}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		out <- StreamChunk{Error: fmt.Errorf("reading stream: %w", err)}
	}
}

// CheckAPIKeyRequired verifies the API key is set if required.
func (b *BaseProvider) CheckAPIKeyRequired() error {
	if b.apiKeyEnv != "" && b.GetAPIKey() == "" {
		return fmt.Errorf("API key not set: please set %s environment variable", b.apiKeyEnv)
	}
	return nil
}
