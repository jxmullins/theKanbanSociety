// Package debate handles the orchestration of multi-AI debates.
package debate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jxmullins/thekanbansociety/internal/config"
	"github.com/jxmullins/thekanbansociety/internal/provider"
)

// Mode represents the debate mode.
type Mode string

const (
	ModeCollaborative Mode = "collaborative"
	ModeAdversarial   Mode = "adversarial"
	ModeSocratic      Mode = "socratic"
)

// Options configures a debate session.
type Options struct {
	Topic     string
	Mode      Mode
	Rounds    int
	Members   []string
	Stream    bool
	Verbose   bool
	OutputDir string
}

// Response holds a single AI's response in a round.
type Response struct {
	AIID      string
	AIName    string
	Content   string
	Round     int
	Phase     string
	Timestamp time.Time
}

// Transcript holds the complete debate record.
type Transcript struct {
	Topic     string
	Mode      Mode
	StartTime time.Time
	EndTime   time.Time
	Members   []string
	Rounds    [][]Response
	Synthesis []Response
	Final     string
}

// Runner orchestrates debate sessions.
type Runner struct {
	registry *provider.Registry
	config   *config.Config
	personas map[string]*config.Persona
}

// NewRunner creates a new debate runner.
func NewRunner(registry *provider.Registry, cfg *config.Config) *Runner {
	return &Runner{
		registry: registry,
		config:   cfg,
		personas: make(map[string]*config.Persona),
	}
}

// Run executes a complete debate session.
func (r *Runner) Run(ctx context.Context, opts Options) error {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("           THE COUNCIL OF LEGENDS")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Printf("Topic: %s\n", opts.Topic)
	fmt.Printf("Mode:  %s\n", opts.Mode)
	fmt.Printf("Rounds: %d\n", opts.Rounds)
	fmt.Printf("Members: %s\n", strings.Join(opts.Members, ", "))
	fmt.Println()

	transcript := &Transcript{
		Topic:     opts.Topic,
		Mode:      opts.Mode,
		StartTime: time.Now(),
		Members:   opts.Members,
		Rounds:    make([][]Response, 0, opts.Rounds),
	}

	// Validate members exist in registry
	for _, member := range opts.Members {
		if _, _, err := r.registry.GetForModel(member); err != nil {
			return fmt.Errorf("member %q: %w", member, err)
		}
	}

	// Round 1: Opening Statements
	openings, err := r.runOpeningRound(ctx, opts)
	if err != nil {
		return fmt.Errorf("opening round: %w", err)
	}
	transcript.Rounds = append(transcript.Rounds, openings)

	// Rounds 2-N: Rebuttals
	for round := 2; round <= opts.Rounds; round++ {
		rebuttals, err := r.runRebuttalRound(ctx, opts, round, transcript)
		if err != nil {
			return fmt.Errorf("round %d: %w", round, err)
		}
		transcript.Rounds = append(transcript.Rounds, rebuttals)
	}

	// Final: Individual Syntheses
	syntheses, err := r.runSynthesisRound(ctx, opts, transcript)
	if err != nil {
		return fmt.Errorf("synthesis round: %w", err)
	}
	transcript.Synthesis = syntheses

	// Combined Final Verdict
	final, err := r.runFinalVerdict(ctx, opts, transcript)
	if err != nil {
		return fmt.Errorf("final verdict: %w", err)
	}
	transcript.Final = final
	transcript.EndTime = time.Now()

	// Save transcript
	if opts.OutputDir != "" {
		if err := r.saveTranscript(opts, transcript); err != nil {
			fmt.Printf("Warning: failed to save transcript: %v\n", err)
		}
	}

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("           DEBATE COMPLETE")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("Duration: %s\n", transcript.EndTime.Sub(transcript.StartTime).Round(time.Second))

	return nil
}

func (r *Runner) runOpeningRound(ctx context.Context, opts Options) ([]Response, error) {
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Println("  Round 1: Opening Statements")
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Println()

	prompt := r.buildOpeningPrompt(opts)
	responses := make([]Response, 0, len(opts.Members))

	for _, member := range opts.Members {
		resp, err := r.invokeAI(ctx, member, prompt, opts)
		if err != nil {
			fmt.Printf("[%s failed: %v]\n\n", member, err)
			continue
		}

		responses = append(responses, Response{
			AIID:      member,
			AIName:    r.getDisplayName(member),
			Content:   resp.Content,
			Round:     1,
			Phase:     "opening",
			Timestamp: time.Now(),
		})
	}

	return responses, nil
}

func (r *Runner) runRebuttalRound(ctx context.Context, opts Options, round int, transcript *Transcript) ([]Response, error) {
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Printf("  Round %d: Rebuttals\n", round)
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Println()

	responses := make([]Response, 0, len(opts.Members))

	for _, member := range opts.Members {
		prompt := r.buildRebuttalPrompt(opts, member, round, transcript)
		resp, err := r.invokeAI(ctx, member, prompt, opts)
		if err != nil {
			fmt.Printf("[%s failed: %v]\n\n", member, err)
			continue
		}

		responses = append(responses, Response{
			AIID:      member,
			AIName:    r.getDisplayName(member),
			Content:   resp.Content,
			Round:     round,
			Phase:     "rebuttal",
			Timestamp: time.Now(),
		})
	}

	return responses, nil
}

func (r *Runner) runSynthesisRound(ctx context.Context, opts Options, transcript *Transcript) ([]Response, error) {
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Println("  Final Round: Individual Syntheses")
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Println()

	responses := make([]Response, 0, len(opts.Members))

	for _, member := range opts.Members {
		prompt := r.buildSynthesisPrompt(opts, member, transcript)
		resp, err := r.invokeAI(ctx, member, prompt, opts)
		if err != nil {
			fmt.Printf("[%s failed: %v]\n\n", member, err)
			continue
		}

		responses = append(responses, Response{
			AIID:      member,
			AIName:    r.getDisplayName(member),
			Content:   resp.Content,
			Round:     0,
			Phase:     "synthesis",
			Timestamp: time.Now(),
		})
	}

	return responses, nil
}

func (r *Runner) runFinalVerdict(ctx context.Context, opts Options, transcript *Transcript) (string, error) {
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Println("  The Council's Final Verdict")
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Println()

	// Use the first member (typically Claude) for the final synthesis
	synthesizer := opts.Members[0]
	prompt := r.buildFinalVerdictPrompt(opts, transcript)

	resp, err := r.invokeAI(ctx, synthesizer, prompt, opts)
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}

func (r *Runner) invokeAI(ctx context.Context, aiID string, prompt string, opts Options) (*provider.Response, error) {
	modelCfg, ok := r.config.GetModel(aiID)
	if !ok {
		return nil, fmt.Errorf("model %q not found in config", aiID)
	}

	displayName := modelCfg.DisplayName
	if displayName == "" {
		displayName = aiID
	}

	fmt.Printf("▶ %s\n", displayName)
	fmt.Println()

	systemPrompt := r.getSystemPrompt(aiID, opts)

	if opts.Stream {
		return r.invokeStreaming(ctx, aiID, prompt, systemPrompt)
	}

	resp, err := r.registry.Invoke(ctx, aiID, provider.Request{
		Prompt:       prompt,
		SystemPrompt: systemPrompt,
	})
	if err != nil {
		return nil, err
	}

	fmt.Println(resp.Content)
	fmt.Println()

	return resp, nil
}

func (r *Runner) invokeStreaming(ctx context.Context, aiID, prompt, systemPrompt string) (*provider.Response, error) {
	stream, err := r.registry.Stream(ctx, aiID, provider.Request{
		Prompt:       prompt,
		SystemPrompt: systemPrompt,
	})
	if err != nil {
		return nil, err
	}

	var content strings.Builder
	for chunk := range stream {
		if chunk.Error != nil {
			return nil, chunk.Error
		}
		if chunk.Content != "" {
			fmt.Print(chunk.Content)
			content.WriteString(chunk.Content)
		}
	}
	fmt.Println()
	fmt.Println()

	return &provider.Response{Content: content.String()}, nil
}

func (r *Runner) getDisplayName(aiID string) string {
	if modelCfg, ok := r.config.GetModel(aiID); ok && modelCfg.DisplayName != "" {
		return modelCfg.DisplayName
	}
	return aiID
}

func (r *Runner) getSystemPrompt(aiID string, opts Options) string {
	modeDesc := "collaborative discussion"
	if opts.Mode == ModeAdversarial {
		modeDesc = "adversarial debate"
	} else if opts.Mode == ModeSocratic {
		modeDesc = "Socratic dialogue"
	}

	return fmt.Sprintf(`You are participating in a multi-AI council %s.
Your role is to provide thoughtful, well-reasoned arguments while engaging constructively with other perspectives.

Guidelines:
- Present clear, logical arguments supported by evidence
- Acknowledge valid points from other participants
- Identify areas of agreement and disagreement
- Propose practical solutions when possible
- Keep responses focused and concise (aim for ~400 words)`, modeDesc)
}

func (r *Runner) buildOpeningPrompt(opts Options) string {
	return fmt.Sprintf(`Topic for debate: %s

This is Round 1: Opening Statements.

Please present your opening position on this topic. Provide:
1. Your main thesis or position
2. Key supporting arguments
3. Any important context or considerations

Remember to be clear, logical, and concise.`, opts.Topic)
}

func (r *Runner) buildRebuttalPrompt(opts Options, aiID string, round int, transcript *Transcript) string {
	var context strings.Builder
	context.WriteString(fmt.Sprintf("Topic: %s\n\n", opts.Topic))
	context.WriteString("Previous discussion:\n\n")

	// Include previous rounds
	for _, roundResponses := range transcript.Rounds {
		for _, resp := range roundResponses {
			context.WriteString(fmt.Sprintf("**%s** (Round %d):\n%s\n\n", resp.AIName, resp.Round, resp.Content))
		}
	}

	return fmt.Sprintf(`%s
This is Round %d: Rebuttals.

Based on the discussion so far:
1. Respond to the strongest arguments made by other participants
2. Strengthen or refine your position
3. Identify any common ground
4. Address any weaknesses in opposing arguments

Be respectful but rigorous in your analysis.`, context.String(), round)
}

func (r *Runner) buildSynthesisPrompt(opts Options, aiID string, transcript *Transcript) string {
	var context strings.Builder
	context.WriteString(fmt.Sprintf("Topic: %s\n\n", opts.Topic))
	context.WriteString("Complete debate history:\n\n")

	for _, roundResponses := range transcript.Rounds {
		for _, resp := range roundResponses {
			context.WriteString(fmt.Sprintf("**%s** (Round %d):\n%s\n\n", resp.AIName, resp.Round, resp.Content))
		}
	}

	return fmt.Sprintf(`%s
This is the Final Round: Synthesis.

Please provide your synthesis of this debate:
1. Summarize the key positions and arguments
2. Identify areas of consensus
3. Note remaining disagreements
4. Offer your final assessment and recommendations`, context.String())
}

func (r *Runner) buildFinalVerdictPrompt(opts Options, transcript *Transcript) string {
	var context strings.Builder
	context.WriteString(fmt.Sprintf("Topic: %s\n\n", opts.Topic))
	context.WriteString("Individual syntheses from each AI:\n\n")

	for _, resp := range transcript.Synthesis {
		context.WriteString(fmt.Sprintf("**%s's Synthesis:**\n%s\n\n", resp.AIName, resp.Content))
	}

	return fmt.Sprintf(`%s
As the synthesizer for The Council of Legends, provide a combined final verdict:

1. **Consensus View**: What do the AIs agree on?
2. **Key Insights**: Most valuable contributions from the debate
3. **Remaining Questions**: Unresolved issues or areas needing further exploration
4. **Recommendation**: Actionable guidance based on the collective wisdom

Write this as a unified council verdict, not as an individual opinion.`, context.String())
}

func (r *Runner) saveTranscript(opts Options, transcript *Transcript) error {
	// Validate and clean output directory path
	cleanDir := filepath.Clean(opts.OutputDir)
	
	// Create output directory with restricted permissions
	if err := os.MkdirAll(cleanDir, 0750); err != nil {
		return err
	}

	// Generate filename with sanitized topic
	timestamp := transcript.StartTime.Format("2006-01-02_15-04-05")
	sanitizedTopic := strings.Map(func(r rune) rune {
		if r == ' ' {
			return '_'
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return -1
	}, opts.Topic)
	if len(sanitizedTopic) > 50 {
		sanitizedTopic = sanitizedTopic[:50]
	}

	// Ensure filename is safe
	safeFilename := fmt.Sprintf("%s_%s.md", timestamp, sanitizedTopic)
	filename := filepath.Join(cleanDir, safeFilename)
	
	// Verify the final path is within the output directory
	absDir, err := filepath.Abs(cleanDir)
	if err != nil {
		return fmt.Errorf("resolving output directory: %w", err)
	}
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return fmt.Errorf("resolving file path: %w", err)
	}
	if !strings.HasPrefix(absPath, absDir+string(filepath.Separator)) && absPath != absDir {
		return fmt.Errorf("path traversal detected in output path")
	}

	// Build markdown content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# Council Debate: %s\n\n", opts.Topic))
	content.WriteString(fmt.Sprintf("**Date:** %s\n", transcript.StartTime.Format("January 2, 2006 3:04 PM")))
	content.WriteString(fmt.Sprintf("**Mode:** %s\n", transcript.Mode))
	content.WriteString(fmt.Sprintf("**Members:** %s\n\n", strings.Join(transcript.Members, ", ")))

	for i, roundResponses := range transcript.Rounds {
		if i == 0 {
			content.WriteString("## Round 1: Opening Statements\n\n")
		} else {
			content.WriteString(fmt.Sprintf("## Round %d: Rebuttals\n\n", i+1))
		}

		for _, resp := range roundResponses {
			content.WriteString(fmt.Sprintf("### %s\n\n%s\n\n", resp.AIName, resp.Content))
		}
	}

	content.WriteString("## Individual Syntheses\n\n")
	for _, resp := range transcript.Synthesis {
		content.WriteString(fmt.Sprintf("### %s\n\n%s\n\n", resp.AIName, resp.Content))
	}

	content.WriteString("## The Council's Final Verdict\n\n")
	content.WriteString(transcript.Final)
	content.WriteString("\n")

	return os.WriteFile(filename, []byte(content.String()), 0640)
}
