// Package team provides multi-AI team collaboration for task completion.
package team

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

// WorkMode represents the team collaboration mode.
type WorkMode string

const (
	ModePairProgramming WorkMode = "pair_programming"
	ModeConsultation    WorkMode = "consultation"
	ModeRoundRobin      WorkMode = "round_robin"
	ModeDivideConquer   WorkMode = "divide_conquer"
	ModeFreeForm        WorkMode = "free_form"
)

// CheckpointLevel controls when user approval is required.
type CheckpointLevel string

const (
	CheckpointAll   CheckpointLevel = "all"
	CheckpointMajor CheckpointLevel = "major"
	CheckpointNone  CheckpointLevel = "none"
)

// Options configures a team session.
type Options struct {
	Task            string
	PM              string          // Forced PM, or empty for auto-selection
	Mode            WorkMode        // Forced mode, or empty for PM decision
	Members         []string        // Team members (excluding PM)
	IncludeArbiter  bool            // Include 4th AI as arbiter
	CheckpointLevel CheckpointLevel
	ShowCosts       bool
	OutputDir       string
	Verbose         bool
}

// Phase represents a team workflow phase.
type Phase int

const (
	PhaseAnalysis Phase = iota
	PhasePlanning
	PhaseExecution
	PhaseReview
	PhaseDelivery
	PhaseComplete
)

func (p Phase) String() string {
	switch p {
	case PhaseAnalysis:
		return "Task Analysis"
	case PhasePlanning:
		return "Planning"
	case PhaseExecution:
		return "Execution"
	case PhaseReview:
		return "Review"
	case PhaseDelivery:
		return "Delivery"
	case PhaseComplete:
		return "Complete"
	default:
		return "Unknown"
	}
}

// Session holds the state of a team collaboration session.
type Session struct {
	ID          string
	Task        string
	PM          string
	Mode        WorkMode
	Members     []string
	Phase       Phase
	StartTime   time.Time
	ProjectDir  string
	Artifacts   []Artifact
	Checkpoints []Checkpoint
	Plan        *Plan
}

// Plan holds the PM's work plan.
type Plan struct {
	Summary     string
	Steps       []PlanStep
	Assignments map[string][]string // AI -> assigned steps
	EstDuration string
}

// PlanStep represents a single step in the plan.
type PlanStep struct {
	ID          string
	Description string
	AssignedTo  string
	DependsOn   []string
	Status      string
}

// Runner orchestrates team collaboration sessions.
type Runner struct {
	registry *provider.Registry
	config   *config.Config
	Events   chan Event // Event channel for TUI
}

// NewRunner creates a new team runner.
func NewRunner(registry *provider.Registry, cfg *config.Config) *Runner {
	return &Runner{
		registry: registry,
		config:   cfg,
		Events:   make(chan Event, 100), // Buffered channel
	}
}

// emit sends an event to the Events channel if it exists and has listeners.
func (r *Runner) emit(event Event) {
	select {
	case r.Events <- event:
	default:
		// Channel full or no listeners, skip
	}
}

// emitTask is a convenience method for task-related events.
func (r *Runner) emitTask(eventType EventType, taskID, actor string, data interface{}) {
	r.emit(NewTaskEvent(eventType, taskID, actor, data))
}

// Run executes a team collaboration session.
func (r *Runner) Run(ctx context.Context, opts Options) error {
	// Create session
	session := &Session{
		ID:        fmt.Sprintf("team_%d", time.Now().Unix()),
		Task:      opts.Task,
		StartTime: time.Now(),
		Phase:     PhaseAnalysis,
		Members:   opts.Members,
	}

	// Print header
	r.printHeader(opts)

	// Emit phase change
	r.emit(NewEvent(EventPhaseChanged, "system", PhaseChangedData{
		OldPhase: -1,
		NewPhase: PhaseAnalysis,
	}))

	// Phase 1: Analyze task and select PM
	pm, err := r.selectPM(ctx, opts, session)
	if err != nil {
		r.emit(NewEvent(EventError, "system", ErrorData{Error: err, Message: "PM selection failed"}))
		return fmt.Errorf("PM selection failed: %w", err)
	}
	session.PM = pm
	r.emit(NewEvent(EventPMSelected, pm, PMSelectedData{
		PMID:        pm,
		DisplayName: r.getDisplayName(pm),
	}))
	fmt.Printf("Project Manager: %s\n\n", r.getDisplayName(pm))

	// Phase 2: PM creates plan and selects work mode
	r.emit(NewEvent(EventPhaseChanged, "system", PhaseChangedData{
		OldPhase: PhaseAnalysis,
		NewPhase: PhasePlanning,
	}))
	session.Phase = PhasePlanning

	plan, mode, err := r.createPlan(ctx, opts, session)
	if err != nil {
		r.emit(NewEvent(EventError, "system", ErrorData{Error: err, Message: "Planning failed"}))
		return fmt.Errorf("planning failed: %w", err)
	}
	session.Plan = plan
	session.Mode = mode

	// Emit PM decision with tasks
	r.emit(NewEvent(EventPMDecision, pm, PMDecisionData{
		WorkMode:    mode,
		PlanSummary: plan.Summary,
		Tasks:       plan.Steps,
	}))

	// Emit task created for each step
	for _, step := range plan.Steps {
		r.emitTask(EventTaskCreated, step.ID, pm, TaskCreatedData{
			Title:       step.Description,
			Description: step.Description,
			AssignedTo:  step.AssignedTo,
			DependsOn:   step.DependsOn,
		})
	}

	fmt.Printf("Work Mode: %s\n", mode)
	fmt.Printf("Plan: %s\n\n", plan.Summary)

	// Checkpoint: Plan approval (if enabled)
	if opts.CheckpointLevel != CheckpointNone {
		if !r.requestApproval(ctx, "Plan Approval", plan.Summary) {
			return fmt.Errorf("plan not approved")
		}
	}

	// Setup project directory
	if opts.OutputDir != "" {
		session.ProjectDir = opts.OutputDir
		if err := os.MkdirAll(session.ProjectDir, 0755); err != nil {
			return fmt.Errorf("creating project directory: %w", err)
		}
	}

	// Phase 3: Execute work based on mode
	r.emit(NewEvent(EventPhaseChanged, "system", PhaseChangedData{
		OldPhase: PhasePlanning,
		NewPhase: PhaseExecution,
	}))
	session.Phase = PhaseExecution

	executor := NewModeExecutor(r.registry, r.config, session)
	executor.events = r.Events // Pass events channel to executor
	artifacts, err := executor.Execute(ctx, opts)
	if err != nil {
		r.emit(NewEvent(EventError, "system", ErrorData{Error: err, Message: "Execution failed"}))
		return fmt.Errorf("execution failed: %w", err)
	}
	session.Artifacts = artifacts

	// Phase 4: Review
	r.emit(NewEvent(EventPhaseChanged, "system", PhaseChangedData{
		OldPhase: PhaseExecution,
		NewPhase: PhaseReview,
	}))
	session.Phase = PhaseReview
	if err := r.runReview(ctx, session); err != nil {
		return fmt.Errorf("review failed: %w", err)
	}

	// Phase 5: Delivery
	r.emit(NewEvent(EventPhaseChanged, "system", PhaseChangedData{
		OldPhase: PhaseReview,
		NewPhase: PhaseDelivery,
	}))
	session.Phase = PhaseDelivery
	if err := r.deliver(ctx, session, opts); err != nil {
		return fmt.Errorf("delivery failed: %w", err)
	}

	session.Phase = PhaseComplete
	r.emit(NewEvent(EventSessionComplete, "system", nil))
	r.printSummary(session)

	return nil
}

func (r *Runner) printHeader(opts Options) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("           THE COUNCIL OF LEGENDS")
	fmt.Println("              Team Collaboration")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Printf("Task: %s\n", opts.Task)
	fmt.Println()
}

func (r *Runner) selectPM(ctx context.Context, opts Options, session *Session) (string, error) {
	// If PM is forced, use that
	if opts.PM != "" {
		return opts.PM, nil
	}

	// Otherwise, analyze task and select best PM
	selector := NewPMSelector(r.registry, r.config)
	return selector.Select(ctx, opts.Task, session.Members)
}

func (r *Runner) createPlan(ctx context.Context, opts Options, session *Session) (*Plan, WorkMode, error) {
	// Have PM analyze task and create plan
	prompt := fmt.Sprintf(`You are the Project Manager for this task. Analyze it and create a work plan.

Task: %s

Team Members: %s

Create a plan with:
1. A brief summary (1-2 sentences)
2. Numbered steps to complete the task
3. Which team member should handle each step
4. Recommended work mode:
   - pair_programming: Two AIs collaborate on same artifact
   - consultation: You lead, others provide input when asked
   - round_robin: Sequential contributions from each member
   - divide_conquer: Split task into parallel subtasks, merge results
   - free_form: Open collaboration

Respond in this format:
SUMMARY: <brief summary>
MODE: <work_mode>
STEPS:
1. <step description> [ASSIGNED: <ai_id>]
2. <step description> [ASSIGNED: <ai_id>]
...`, opts.Task, strings.Join(session.Members, ", "))

	resp, err := r.registry.Invoke(ctx, session.PM, provider.Request{
		Prompt:       prompt,
		SystemPrompt: "You are an expert project manager coordinating a team of AI assistants.",
	})
	if err != nil {
		return nil, "", err
	}

	// Parse response
	plan, mode := r.parsePlanResponse(resp.Content, opts)

	return plan, mode, nil
}

func (r *Runner) parsePlanResponse(content string, opts Options) (*Plan, WorkMode) {
	plan := &Plan{
		Summary:     "Task execution plan",
		Steps:       []PlanStep{},
		Assignments: make(map[string][]string),
	}

	mode := ModeFreeForm
	if opts.Mode != "" {
		mode = opts.Mode
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "SUMMARY:") {
			plan.Summary = strings.TrimPrefix(line, "SUMMARY:")
			plan.Summary = strings.TrimSpace(plan.Summary)
		} else if strings.HasPrefix(line, "MODE:") {
			if opts.Mode == "" { // Only use PM's mode if not forced
				modeStr := strings.TrimPrefix(line, "MODE:")
				modeStr = strings.TrimSpace(modeStr)
				switch modeStr {
				case "pair_programming":
					mode = ModePairProgramming
				case "consultation":
					mode = ModeConsultation
				case "round_robin":
					mode = ModeRoundRobin
				case "divide_conquer":
					mode = ModeDivideConquer
				case "free_form":
					mode = ModeFreeForm
				}
			}
		} else if len(line) > 2 && line[0] >= '1' && line[0] <= '9' && line[1] == '.' {
			// Parse step
			step := PlanStep{
				ID:          fmt.Sprintf("step_%d", len(plan.Steps)+1),
				Description: line,
				Status:      "pending",
			}

			// Extract assignment if present
			if idx := strings.Index(line, "[ASSIGNED:"); idx != -1 {
				end := strings.Index(line[idx:], "]")
				if end != -1 {
					assigned := line[idx+10 : idx+end]
					step.AssignedTo = strings.TrimSpace(assigned)
					step.Description = strings.TrimSpace(line[:idx])
				}
			}

			plan.Steps = append(plan.Steps, step)
		}
	}

	return plan, mode
}

func (r *Runner) runReview(ctx context.Context, session *Session) error {
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Println("  Review Phase")
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Println()

	// Have PM review the artifacts
	var artifactSummary strings.Builder
	for _, a := range session.Artifacts {
		artifactSummary.WriteString(fmt.Sprintf("- %s: %s\n", a.Name, a.Description))
	}

	prompt := fmt.Sprintf(`Review the completed work for this task:

Task: %s

Artifacts created:
%s

Provide a brief review:
1. What was accomplished
2. Any issues or concerns
3. Recommendations`, session.Task, artifactSummary.String())

	resp, err := r.registry.Invoke(ctx, session.PM, provider.Request{
		Prompt:       prompt,
		SystemPrompt: "You are reviewing work completed by your team.",
	})
	if err != nil {
		return err
	}

	fmt.Printf("PM Review:\n%s\n\n", resp.Content)
	return nil
}

func (r *Runner) deliver(ctx context.Context, session *Session, opts Options) error {
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Println("  Delivery")
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Println()

	// Save artifacts if project directory is set
	if session.ProjectDir != "" {
		for _, artifact := range session.Artifacts {
			if err := artifact.Save(session.ProjectDir); err != nil {
				fmt.Printf("Warning: failed to save %s: %v\n", artifact.Name, err)
			} else {
				fmt.Printf("Saved: %s\n", filepath.Join(session.ProjectDir, artifact.Name))
			}
		}
	}

	// Save session summary
	summaryPath := filepath.Join(session.ProjectDir, "SESSION_SUMMARY.md")
	summary := r.generateSessionSummary(session)
	if err := os.WriteFile(summaryPath, []byte(summary), 0644); err != nil {
		fmt.Printf("Warning: failed to save summary: %v\n", err)
	} else {
		fmt.Printf("Saved: %s\n", summaryPath)
	}

	return nil
}

func (r *Runner) generateSessionSummary(session *Session) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Team Session: %s\n\n", session.ID))
	b.WriteString(fmt.Sprintf("**Task:** %s\n\n", session.Task))
	b.WriteString(fmt.Sprintf("**Project Manager:** %s\n", session.PM))
	b.WriteString(fmt.Sprintf("**Work Mode:** %s\n", session.Mode))
	b.WriteString(fmt.Sprintf("**Team:** %s\n\n", strings.Join(session.Members, ", ")))
	b.WriteString(fmt.Sprintf("**Started:** %s\n", session.StartTime.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("**Duration:** %s\n\n", time.Since(session.StartTime).Round(time.Second)))

	b.WriteString("## Plan\n\n")
	b.WriteString(session.Plan.Summary + "\n\n")

	b.WriteString("## Artifacts\n\n")
	for _, a := range session.Artifacts {
		b.WriteString(fmt.Sprintf("- **%s**: %s\n", a.Name, a.Description))
	}

	return b.String()
}

func (r *Runner) printSummary(session *Session) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("           SESSION COMPLETE")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("Duration: %s\n", time.Since(session.StartTime).Round(time.Second))
	fmt.Printf("Artifacts: %d\n", len(session.Artifacts))
	if session.ProjectDir != "" {
		fmt.Printf("Output: %s\n", session.ProjectDir)
	}
	fmt.Println()
}

func (r *Runner) requestApproval(ctx context.Context, checkpoint, description string) bool {
	fmt.Printf("\n[Checkpoint: %s]\n", checkpoint)
	fmt.Printf("%s\n", description)
	fmt.Print("\nApprove? (y/n): ")

	// In a real implementation, this would read user input
	// For now, auto-approve
	fmt.Println("y (auto-approved)")
	return true
}

func (r *Runner) getDisplayName(aiID string) string {
	if modelCfg, ok := r.config.GetModel(aiID); ok && modelCfg.DisplayName != "" {
		return modelCfg.DisplayName
	}
	return aiID
}
