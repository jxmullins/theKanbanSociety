// Package provider provides AI provider adapters.
package provider

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// CLIProvider wraps a CLI tool as a provider.
type CLIProvider struct {
	name        string
	command     string
	args        []string // Base args before prompt
	promptFlag  string   // Flag to pass prompt (e.g., "-p")
	systemFlag  string   // Flag for system prompt if supported
	streamable  bool
}

// CLIProviderConfig configures a CLI provider.
type CLIProviderConfig struct {
	Name       string
	Command    string
	Args       []string
	PromptFlag string
	SystemFlag string
	Streamable bool
}

// NewCLIProvider creates a new CLI-based provider.
func NewCLIProvider(cfg CLIProviderConfig) *CLIProvider {
	return &CLIProvider{
		name:       cfg.Name,
		command:    cfg.Command,
		args:       cfg.Args,
		promptFlag: cfg.PromptFlag,
		systemFlag: cfg.SystemFlag,
		streamable: cfg.Streamable,
	}
}

// Name returns the provider name.
func (p *CLIProvider) Name() string {
	return p.name
}

// Models returns available model IDs for this provider.
func (p *CLIProvider) Models() []string {
	return []string{p.name}
}

// Invoke calls the CLI and returns the response.
func (p *CLIProvider) Invoke(ctx context.Context, req Request) (*Response, error) {
	args := append([]string{}, p.args...)

	// Add system prompt if supported and provided
	if p.systemFlag != "" && req.SystemPrompt != "" {
		args = append(args, p.systemFlag, req.SystemPrompt)
	}

	// Add prompt
	if p.promptFlag != "" {
		args = append(args, p.promptFlag, req.Prompt)
	} else {
		args = append(args, req.Prompt)
	}

	cmd := exec.CommandContext(ctx, p.command, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s CLI error: %w (stderr: %s)", p.name, err, stderr.String())
	}

	return &Response{
		Content:    strings.TrimSpace(stdout.String()),
		TokensUsed: 0, // CLI doesn't report tokens
	}, nil
}

// Stream calls the CLI and streams the response line by line.
func (p *CLIProvider) Stream(ctx context.Context, req Request) (<-chan StreamChunk, error) {
	args := append([]string{}, p.args...)

	// Add system prompt if supported and provided
	if p.systemFlag != "" && req.SystemPrompt != "" {
		args = append(args, p.systemFlag, req.SystemPrompt)
	}

	// Add prompt
	if p.promptFlag != "" {
		args = append(args, p.promptFlag, req.Prompt)
	} else {
		args = append(args, req.Prompt)
	}

	cmd := exec.CommandContext(ctx, p.command, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting %s CLI: %w", p.name, err)
	}

	ch := make(chan StreamChunk, 10)

	go func() {
		defer close(ch)
		defer cmd.Wait()

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				cmd.Process.Kill()
				return
			case ch <- StreamChunk{Content: scanner.Text() + "\n"}:
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- StreamChunk{Error: err}
		}
	}()

	return ch, nil
}

// HealthCheck verifies the CLI is available.
func (p *CLIProvider) HealthCheck(ctx context.Context) error {
	_, err := exec.LookPath(p.command)
	if err != nil {
		return fmt.Errorf("%s CLI not found: %w", p.name, err)
	}
	return nil
}

// NewClaudeCLIProvider creates a provider using the Claude Code CLI.
func NewClaudeCLIProvider() *CLIProvider {
	return NewCLIProvider(CLIProviderConfig{
		Name:       "claude-cli",
		Command:    "claude",
		Args:       []string{"--dangerously-skip-permissions", "-p"},
		PromptFlag: "", // Prompt is positional after -p
		SystemFlag: "--append-system-prompt",
		Streamable: true,
	})
}

// NewGeminiCLIProvider creates a provider using the Gemini CLI.
func NewGeminiCLIProvider() *CLIProvider {
	return NewCLIProvider(CLIProviderConfig{
		Name:       "gemini-cli",
		Command:    "gemini",
		Args:       []string{"--yolo"},
		PromptFlag: "-p",
		Streamable: true,
	})
}

// NewCodexCLIProvider creates a provider using the OpenAI Codex CLI.
func NewCodexCLIProvider() *CLIProvider {
	return NewCLIProvider(CLIProviderConfig{
		Name:       "codex-cli",
		Command:    "codex",
		Args:       []string{"exec"},
		PromptFlag: "", // Prompt is passed as positional arg
		Streamable: true,
	})
}

// DetectCLIProviders returns providers for all available CLI tools.
func DetectCLIProviders() []Provider {
	var providers []Provider

	// Check for claude CLI
	if _, err := exec.LookPath("claude"); err == nil {
		providers = append(providers, NewClaudeCLIProvider())
	}

	// Check for gemini CLI
	if _, err := exec.LookPath("gemini"); err == nil {
		providers = append(providers, NewGeminiCLIProvider())
	}

	// Check for codex CLI (OpenAI)
	if _, err := exec.LookPath("codex"); err == nil {
		providers = append(providers, NewCodexCLIProvider())
	}

	return providers
}
