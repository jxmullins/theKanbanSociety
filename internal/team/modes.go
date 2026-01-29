package team

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/jxmullins/thekanbansociety/internal/config"
	"github.com/jxmullins/thekanbansociety/internal/provider"
)

// ModeExecutor executes work based on the selected mode.
type ModeExecutor struct {
	registry *provider.Registry
	config   *config.Config
	session  *Session
	events   chan Event // Event channel for TUI
}

// emit sends an event to the events channel.
func (e *ModeExecutor) emit(event Event) {
	if e.events != nil {
		select {
		case e.events <- event:
		default:
		}
	}
}

// emitTask emits a task-related event.
func (e *ModeExecutor) emitTask(eventType EventType, taskID, actor string, data interface{}) {
	e.emit(NewTaskEvent(eventType, taskID, actor, data))
}

// NewModeExecutor creates a new mode executor.
func NewModeExecutor(registry *provider.Registry, cfg *config.Config, session *Session) *ModeExecutor {
	return &ModeExecutor{
		registry: registry,
		config:   cfg,
		session:  session,
	}
}

// Execute runs the work using the session's selected mode.
func (e *ModeExecutor) Execute(ctx context.Context, opts Options) ([]Artifact, error) {
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Printf("  Execution Phase (%s)\n", e.session.Mode)
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Println()

	switch e.session.Mode {
	case ModePairProgramming:
		return e.executePairProgramming(ctx, opts)
	case ModeConsultation:
		return e.executeConsultation(ctx, opts)
	case ModeRoundRobin:
		return e.executeRoundRobin(ctx, opts)
	case ModeDivideConquer:
		return e.executeDivideConquer(ctx, opts)
	case ModeFreeForm:
		return e.executeFreeForm(ctx, opts)
	default:
		return e.executeFreeForm(ctx, opts)
	}
}

// executePairProgramming runs pair programming mode.
// Two AIs collaborate on the same artifact, taking turns.
func (e *ModeExecutor) executePairProgramming(ctx context.Context, opts Options) ([]Artifact, error) {
	fmt.Println("Mode: Pair Programming")
	fmt.Println("Two AIs collaborate on the same artifact")

	if len(e.session.Members) < 2 {
		return nil, fmt.Errorf("pair programming requires at least 2 team members")
	}

	// Select pair (PM + first member, or first two members)
	driver := e.session.Members[0]
	navigator := e.session.Members[1]
	if driver == e.session.PM && len(e.session.Members) > 2 {
		navigator = e.session.Members[2]
	}

	fmt.Printf("Driver: %s\n", e.getDisplayName(driver))
	fmt.Printf("Navigator: %s\n\n", e.getDisplayName(navigator))

	var artifacts []Artifact
	var currentWork strings.Builder

	// Iterate: driver writes, navigator reviews
	for i := 0; i < 3; i++ {
		// Driver's turn
		fmt.Printf("Turn %d - %s (Driver):\n", i+1, e.getDisplayName(driver))

		prompt := fmt.Sprintf(`You are the driver in pair programming.
Task: %s

%s

Write or continue the implementation. Be specific and produce actual code/content.`,
			e.session.Task,
			e.buildContext(currentWork.String(), i))

		resp, err := e.registry.Invoke(ctx, driver, provider.Request{
			Prompt:       prompt,
			SystemPrompt: "You are a skilled developer working in pair programming mode.",
		})
		if err != nil {
			return nil, fmt.Errorf("driver failed: %w", err)
		}

		fmt.Println(truncateOutput(resp.Content, 500))
		currentWork.WriteString("\n\n### Driver Output:\n")
		currentWork.WriteString(resp.Content)

		// Navigator reviews
		fmt.Printf("\n%s (Navigator) reviewing...\n", e.getDisplayName(navigator))

		reviewPrompt := fmt.Sprintf(`You are the navigator reviewing the driver's work.
Task: %s

Driver's output:
%s

Review and suggest improvements. Point out any issues or optimizations.`,
			e.session.Task, resp.Content)

		reviewResp, err := e.registry.Invoke(ctx, navigator, provider.Request{
			Prompt:       reviewPrompt,
			SystemPrompt: "You are a code reviewer in pair programming mode.",
		})
		if err != nil {
			fmt.Printf("Navigator review failed: %v\n", err)
		} else {
			fmt.Println(truncateOutput(reviewResp.Content, 300))
			currentWork.WriteString("\n\n### Navigator Review:\n")
			currentWork.WriteString(reviewResp.Content)
		}

		fmt.Println()

		// Swap roles for next iteration
		driver, navigator = navigator, driver
	}

	artifacts = append(artifacts, Artifact{
		Name:        "pair_output.md",
		Type:        ArtifactCode,
		Content:     currentWork.String(),
		Description: "Pair programming session output",
		CreatedBy:   fmt.Sprintf("%s, %s", e.session.Members[0], e.session.Members[1]),
	})

	return artifacts, nil
}

// executeConsultation runs consultation mode.
// PM leads and consults team members for specific input.
func (e *ModeExecutor) executeConsultation(ctx context.Context, opts Options) ([]Artifact, error) {
	fmt.Println("Mode: Consultation")
	fmt.Printf("PM %s leads, consulting team as needed\n\n", e.getDisplayName(e.session.PM))

	var artifacts []Artifact
	var context strings.Builder

	// PM starts with initial approach
	fmt.Printf("%s (PM) - Initial Approach:\n", e.getDisplayName(e.session.PM))

	initialPrompt := fmt.Sprintf(`You are the PM leading this task. Create an initial approach.
Task: %s

Outline your approach and identify what input you need from team members.`,
		e.session.Task)

	resp, err := e.registry.Invoke(ctx, e.session.PM, provider.Request{
		Prompt:       initialPrompt,
		SystemPrompt: "You are a project manager leading a team.",
	})
	if err != nil {
		return nil, err
	}

	fmt.Println(truncateOutput(resp.Content, 500))
	context.WriteString("PM Initial Approach:\n")
	context.WriteString(resp.Content)
	fmt.Println()

	// Consult each team member
	for _, member := range e.session.Members {
		if member == e.session.PM {
			continue
		}

		fmt.Printf("Consulting %s:\n", e.getDisplayName(member))

		consultPrompt := fmt.Sprintf(`The PM has this approach for the task:
%s

Task: %s

Provide your expert input, suggestions, or concerns.`,
			context.String(), e.session.Task)

		memberResp, err := e.registry.Invoke(ctx, member, provider.Request{
			Prompt:       consultPrompt,
			SystemPrompt: "You are a team member providing consultation.",
		})
		if err != nil {
			fmt.Printf("Consultation failed: %v\n", err)
			continue
		}

		fmt.Println(truncateOutput(memberResp.Content, 400))
		context.WriteString(fmt.Sprintf("\n\n%s's Input:\n%s", e.getDisplayName(member), memberResp.Content))
		fmt.Println()
	}

	// PM synthesizes and produces final output
	fmt.Printf("%s (PM) - Final Output:\n", e.getDisplayName(e.session.PM))

	finalPrompt := fmt.Sprintf(`Based on the consultations:
%s

Produce the final deliverable for: %s`,
		context.String(), e.session.Task)

	finalResp, err := e.registry.Invoke(ctx, e.session.PM, provider.Request{
		Prompt:       finalPrompt,
		SystemPrompt: "You are producing the final deliverable incorporating team input.",
	})
	if err != nil {
		return nil, err
	}

	fmt.Println(truncateOutput(finalResp.Content, 500))

	artifacts = append(artifacts, Artifact{
		Name:        "consultation_output.md",
		Type:        ArtifactDocument,
		Content:     finalResp.Content,
		Description: "Final output from consultation",
		CreatedBy:   e.session.PM,
	})

	return artifacts, nil
}

// executeRoundRobin runs round-robin mode.
// Each AI contributes sequentially.
func (e *ModeExecutor) executeRoundRobin(ctx context.Context, opts Options) ([]Artifact, error) {
	fmt.Println("Mode: Round Robin")
	fmt.Println("Sequential contributions from each member")

	var artifacts []Artifact
	var accumulated strings.Builder

	for round := 1; round <= 2; round++ {
		fmt.Printf("--- Round %d ---\n\n", round)

		for _, member := range e.session.Members {
			fmt.Printf("%s's contribution:\n", e.getDisplayName(member))

			prompt := fmt.Sprintf(`Task: %s

Previous contributions:
%s

Add your contribution. Build on what others have done.`,
				e.session.Task, accumulated.String())

			resp, err := e.registry.Invoke(ctx, member, provider.Request{
				Prompt:       prompt,
				SystemPrompt: "You are contributing to a collaborative project.",
			})
			if err != nil {
				fmt.Printf("Failed: %v\n", err)
				continue
			}

			fmt.Println(truncateOutput(resp.Content, 400))
			accumulated.WriteString(fmt.Sprintf("\n\n### %s (Round %d):\n%s",
				e.getDisplayName(member), round, resp.Content))
			fmt.Println()
		}
	}

	artifacts = append(artifacts, Artifact{
		Name:        "round_robin_output.md",
		Type:        ArtifactDocument,
		Content:     accumulated.String(),
		Description: "Combined round-robin contributions",
		CreatedBy:   strings.Join(e.session.Members, ", "),
	})

	return artifacts, nil
}

// executeDivideConquer runs divide-and-conquer mode.
// Split task, parallel work, merge results.
func (e *ModeExecutor) executeDivideConquer(ctx context.Context, opts Options) ([]Artifact, error) {
	fmt.Println("Mode: Divide and Conquer")
	fmt.Println("Parallel subtasks, then merge")

	// PM divides the task
	fmt.Printf("%s (PM) dividing task...\n", e.getDisplayName(e.session.PM))

	dividePrompt := fmt.Sprintf(`Divide this task into %d independent subtasks:
Task: %s

List each subtask clearly numbered.`,
		len(e.session.Members), e.session.Task)

	divideResp, err := e.registry.Invoke(ctx, e.session.PM, provider.Request{
		Prompt: dividePrompt,
	})
	if err != nil {
		return nil, err
	}

	fmt.Println(divideResp.Content)
	fmt.Println()

	// Execute subtasks in parallel
	var wg sync.WaitGroup
	results := make([]string, len(e.session.Members))
	errors := make([]error, len(e.session.Members))

	for i, member := range e.session.Members {
		wg.Add(1)
		taskID := fmt.Sprintf("subtask_%d", i+1)

		// Emit task started
		e.emitTask(EventTaskStarted, taskID, member, nil)

		go func(idx int, m string, tid string) {
			defer wg.Done()

			fmt.Printf("%s working on subtask %d...\n", e.getDisplayName(m), idx+1)

			subtaskPrompt := fmt.Sprintf(`Complete your assigned subtask.
Main task: %s
Your subtask number: %d
Subtask breakdown: %s`,
				e.session.Task, idx+1, divideResp.Content)

			// Use streaming to emit progress
			streamCh, err := e.registry.Stream(ctx, m, provider.Request{
				Prompt: subtaskPrompt,
			})
			if err != nil {
				errors[idx] = err
				e.emitTask(EventError, tid, m, ErrorData{Error: err, TaskID: tid})
				return
			}

			var content strings.Builder
			for chunk := range streamCh {
				if chunk.Error != nil {
					errors[idx] = chunk.Error
					return
				}
				content.WriteString(chunk.Content)
				e.emitTask(EventTaskProgress, tid, m, TaskProgressData{
					Content:  chunk.Content,
					Progress: 0.5, // Could calculate based on expected length
				})
			}

			results[idx] = fmt.Sprintf("### %s (Subtask %d):\n%s", e.getDisplayName(m), idx+1, content.String())
			e.emitTask(EventTaskCompleted, tid, m, nil)
		}(i, member, taskID)
	}

	wg.Wait()

	// Check for errors
	for i, err := range errors {
		if err != nil {
			fmt.Printf("Subtask %d failed: %v\n", i+1, err)
		}
	}

	// Merge results
	fmt.Printf("\n%s (PM) merging results...\n", e.getDisplayName(e.session.PM))

	var allResults strings.Builder
	for _, r := range results {
		if r != "" {
			allResults.WriteString(r)
			allResults.WriteString("\n\n")
		}
	}

	mergePrompt := fmt.Sprintf(`Merge these subtask results into a cohesive deliverable:
%s

Original task: %s`,
		allResults.String(), e.session.Task)

	mergeResp, err := e.registry.Invoke(ctx, e.session.PM, provider.Request{
		Prompt: mergePrompt,
	})
	if err != nil {
		return nil, err
	}

	fmt.Println(truncateOutput(mergeResp.Content, 500))

	return []Artifact{{
		Name:        "merged_output.md",
		Type:        ArtifactDocument,
		Content:     mergeResp.Content,
		Description: "Merged divide-and-conquer output",
		CreatedBy:   e.session.PM,
	}}, nil
}

// executeFreeForm runs free-form collaboration.
// Open collaboration with PM moderation.
func (e *ModeExecutor) executeFreeForm(ctx context.Context, opts Options) ([]Artifact, error) {
	fmt.Println("Mode: Free Form")
	fmt.Println("Open collaboration with PM moderation")

	var artifacts []Artifact
	var discussion strings.Builder

	// Initial brainstorm from all members
	fmt.Println("--- Brainstorming Phase ---")

	for _, member := range e.session.Members {
		fmt.Printf("%s's thoughts:\n", e.getDisplayName(member))

		prompt := fmt.Sprintf(`Task: %s

Share your initial thoughts, ideas, and approach for this task.`,
			e.session.Task)

		resp, err := e.registry.Invoke(ctx, member, provider.Request{
			Prompt: prompt,
		})
		if err != nil {
			fmt.Printf("Failed: %v\n", err)
			continue
		}

		fmt.Println(truncateOutput(resp.Content, 300))
		discussion.WriteString(fmt.Sprintf("### %s:\n%s\n\n", e.getDisplayName(member), resp.Content))
		fmt.Println()
	}

	// PM synthesizes and directs
	fmt.Println("\n--- PM Direction ---")
	fmt.Printf("%s synthesizing discussion...\n", e.getDisplayName(e.session.PM))

	synthesisPrompt := fmt.Sprintf(`Team discussion:
%s

As PM, synthesize these ideas and provide direction for the final deliverable.
Task: %s`,
		discussion.String(), e.session.Task)

	synthResp, err := e.registry.Invoke(ctx, e.session.PM, provider.Request{
		Prompt: synthesisPrompt,
	})
	if err != nil {
		return nil, err
	}

	fmt.Println(truncateOutput(synthResp.Content, 500))

	// Final collaborative output
	fmt.Println("\n--- Final Output ---")

	finalPrompt := fmt.Sprintf(`Based on the team discussion and PM direction:
%s

%s

Produce the final deliverable.`,
		discussion.String(), synthResp.Content)

	finalResp, err := e.registry.Invoke(ctx, e.session.PM, provider.Request{
		Prompt: finalPrompt,
	})
	if err != nil {
		return nil, err
	}

	fmt.Println(truncateOutput(finalResp.Content, 500))

	artifacts = append(artifacts, Artifact{
		Name:        "freeform_output.md",
		Type:        ArtifactDocument,
		Content:     finalResp.Content,
		Description: "Free-form collaboration output",
		CreatedBy:   strings.Join(e.session.Members, ", "),
	})

	return artifacts, nil
}

func (e *ModeExecutor) getDisplayName(aiID string) string {
	if modelCfg, ok := e.config.GetModel(aiID); ok && modelCfg.DisplayName != "" {
		return modelCfg.DisplayName
	}
	return aiID
}

func (e *ModeExecutor) buildContext(current string, iteration int) string {
	if current == "" {
		return "This is the start. Begin the implementation."
	}
	return fmt.Sprintf("Current progress (iteration %d):\n%s\n\nContinue from here.", iteration, current)
}

func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... (truncated)"
}
