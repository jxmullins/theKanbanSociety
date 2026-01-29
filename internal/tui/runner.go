package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jxmullins/thekanbansociety/internal/config"
	"github.com/jxmullins/thekanbansociety/internal/provider"
)

// Runner manages the TUI debate execution.
type Runner struct {
	registry *provider.Registry
	config   *config.Config
	program  *tea.Program
	model    *Model
}

// NewRunner creates a new TUI runner.
func NewRunner(registry *provider.Registry, cfg *config.Config) *Runner {
	return &Runner{
		registry: registry,
		config:   cfg,
	}
}

// Run starts the TUI debate.
func (r *Runner) Run(opts Options) error {
	model := NewModel(opts, r.config, r.registry)
	r.model = &model

	p := tea.NewProgram(model, tea.WithAltScreen())
	r.program = p

	// Start debate orchestration in background
	go r.orchestrateDebate(opts)

	_, err := p.Run()
	return err
}

// orchestrateDebate manages the debate flow and sends updates to the TUI.
func (r *Runner) orchestrateDebate(opts Options) {
	ctx := context.Background()

	// Wait for TUI to be ready
	// In a real implementation, we'd use a channel

	// Phase 1: Opening statements
	r.runPhase(ctx, opts, PhaseOpening, 1, func(aiID string) string {
		return buildOpeningPrompt(opts.Topic, opts.Mode)
	})

	// Phase 2-N: Rebuttals
	for round := 2; round <= opts.Rounds; round++ {
		r.runPhase(ctx, opts, PhaseRebuttal, round, func(aiID string) string {
			return r.buildRebuttalPrompt(opts.Topic, opts.Mode, round, aiID)
		})
	}

	// Phase: Synthesis
	r.runPhase(ctx, opts, PhaseSynthesis, 0, func(aiID string) string {
		return r.buildSynthesisPrompt(opts.Topic, aiID)
	})

	// Phase: Final verdict (only first AI)
	r.runFinalPhase(ctx, opts)

	// Signal completion
	r.program.Send(DebateCompleteMsg{})
}

// runPhase executes a single debate phase for all AIs.
func (r *Runner) runPhase(ctx context.Context, opts Options, phase Phase, round int, promptBuilder func(string) string) {
	// Clear panels for new phase
	for _, aiID := range opts.Members {
		r.program.Send(StreamChunkMsg{AIID: aiID, Content: "", Done: false})
	}

	// Run each AI sequentially (could be parallelized)
	for _, aiID := range opts.Members {
		r.streamAI(ctx, aiID, promptBuilder(aiID))
	}

	r.program.Send(PhaseCompleteMsg{Phase: phase})
}

// runFinalPhase runs the final verdict using the first AI.
func (r *Runner) runFinalPhase(ctx context.Context, opts Options) {
	if len(opts.Members) == 0 {
		return
	}

	aiID := opts.Members[0]
	prompt := r.buildFinalPrompt(opts.Topic)

	r.streamAI(ctx, aiID, prompt)
	r.program.Send(PhaseCompleteMsg{Phase: PhaseFinal})
}

// streamAI streams a response from a single AI.
func (r *Runner) streamAI(ctx context.Context, aiID, prompt string) {
	// Signal streaming start
	r.program.Send(StreamChunkMsg{AIID: aiID, Content: ""})

	// Mark as streaming (by sending empty content, the model knows it's active)
	systemPrompt := r.getSystemPrompt()

	stream, err := r.registry.Stream(ctx, aiID, provider.Request{
		Prompt:       prompt,
		SystemPrompt: systemPrompt,
	})

	if err != nil {
		r.program.Send(StreamChunkMsg{
			AIID:  aiID,
			Error: err,
			Done:  true,
		})
		return
	}

	for chunk := range stream {
		if chunk.Error != nil {
			r.program.Send(StreamChunkMsg{
				AIID:  aiID,
				Error: chunk.Error,
				Done:  true,
			})
			return
		}

		if chunk.Content != "" {
			r.program.Send(StreamChunkMsg{
				AIID:    aiID,
				Content: chunk.Content,
			})
		}

		if chunk.Done {
			r.program.Send(StreamChunkMsg{
				AIID: aiID,
				Done: true,
			})
			return
		}
	}

	// Stream ended
	r.program.Send(StreamChunkMsg{
		AIID: aiID,
		Done: true,
	})
}

func (r *Runner) getSystemPrompt() string {
	return `You are participating in a multi-AI council debate.
Your role is to provide thoughtful, well-reasoned arguments while engaging constructively with other perspectives.

Guidelines:
- Present clear, logical arguments supported by evidence
- Acknowledge valid points from other participants
- Identify areas of agreement and disagreement
- Propose practical solutions when possible
- Keep responses focused and concise (aim for ~400 words)`
}

func buildOpeningPrompt(topic, mode string) string {
	return fmt.Sprintf(`Topic for debate: %s
Mode: %s

This is Round 1: Opening Statements.

Please present your opening position on this topic. Provide:
1. Your main thesis or position
2. Key supporting arguments
3. Any important context or considerations

Remember to be clear, logical, and concise.`, topic, mode)
}

func (r *Runner) buildRebuttalPrompt(topic, mode string, round int, aiID string) string {
	// In a full implementation, we'd include previous responses
	var context strings.Builder
	context.WriteString(fmt.Sprintf("Topic: %s\n", topic))
	context.WriteString(fmt.Sprintf("Mode: %s\n\n", mode))
	context.WriteString("Previous discussion has covered various perspectives.\n\n")

	return fmt.Sprintf(`%s
This is Round %d: Rebuttals.

Based on the discussion so far:
1. Respond to the strongest arguments made by other participants
2. Strengthen or refine your position
3. Identify any common ground
4. Address any weaknesses in opposing arguments

Be respectful but rigorous in your analysis.`, context.String(), round)
}

func (r *Runner) buildSynthesisPrompt(topic string, aiID string) string {
	return fmt.Sprintf(`Topic: %s

This is the Final Round: Synthesis.

Please provide your synthesis of this debate:
1. Summarize the key positions and arguments
2. Identify areas of consensus
3. Note remaining disagreements
4. Offer your final assessment and recommendations`, topic)
}

func (r *Runner) buildFinalPrompt(topic string) string {
	return fmt.Sprintf(`Topic: %s

As the synthesizer for The Council of Legends, provide a combined final verdict:

1. **Consensus View**: What do the AIs agree on?
2. **Key Insights**: Most valuable contributions from the debate
3. **Remaining Questions**: Unresolved issues or areas needing further exploration
4. **Recommendation**: Actionable guidance based on the collective wisdom

Write this as a unified council verdict, not as an individual opinion.`, topic)
}
