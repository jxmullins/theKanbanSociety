# Team TUI Kanban Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an interactive Kanban-based TUI to the team CLI for tracking AI tasks with user task support.

**Architecture:** Event-driven team package emitting events that the TUI subscribes to. Kanban board displays task cards in columns (Backlog/InProgress/Review/Done) with streaming content, PM header section, and active panel. User can pause and intervene.

**Tech Stack:** Go, Bubble Tea, Lip Gloss, Huh? (forms), existing team package

---

## Task 1: Create Event Types

**Files:**
- Create: `internal/team/events.go`

**Step 1: Create the events file**

```go
// Package team provides multi-AI team collaboration for task completion.
package team

import "time"

// EventType represents the type of team event.
type EventType int

const (
	EventPhaseChanged EventType = iota
	EventPMDecision
	EventTaskCreated
	EventTaskStarted
	EventTaskProgress
	EventTaskCompleted
	EventTaskMovedToReview
	EventPMApproved
	EventUserTaskCreated
	EventUserTaskBlocking
	EventUserTaskCompleted
	EventError
	EventSessionComplete
)

func (e EventType) String() string {
	switch e {
	case EventPhaseChanged:
		return "PhaseChanged"
	case EventPMDecision:
		return "PMDecision"
	case EventTaskCreated:
		return "TaskCreated"
	case EventTaskStarted:
		return "TaskStarted"
	case EventTaskProgress:
		return "TaskProgress"
	case EventTaskCompleted:
		return "TaskCompleted"
	case EventTaskMovedToReview:
		return "TaskMovedToReview"
	case EventPMApproved:
		return "PMApproved"
	case EventUserTaskCreated:
		return "UserTaskCreated"
	case EventUserTaskBlocking:
		return "UserTaskBlocking"
	case EventUserTaskCompleted:
		return "UserTaskCompleted"
	case EventError:
		return "Error"
	case EventSessionComplete:
		return "SessionComplete"
	default:
		return "Unknown"
	}
}

// Event represents a team workflow event.
type Event struct {
	Type      EventType
	Timestamp time.Time
	TaskID    string
	Actor     string      // "pm", AI ID, or "user"
	Data      interface{}
}

// TaskCreatedData contains data for TaskCreated events.
type TaskCreatedData struct {
	Title       string
	Description string
	AssignedTo  string
	IsBlocking  bool   // Only for user tasks
	DependsOn   []string
}

// TaskProgressData contains data for TaskProgress events.
type TaskProgressData struct {
	Content  string  // Streaming chunk
	Progress float64 // 0.0-1.0
}

// PMDecisionData contains data for PMDecision events.
type PMDecisionData struct {
	WorkMode    WorkMode
	PlanSummary string
	Tasks       []PlanStep
}

// PhaseChangedData contains data for PhaseChanged events.
type PhaseChangedData struct {
	OldPhase Phase
	NewPhase Phase
}

// ErrorData contains data for Error events.
type ErrorData struct {
	Error   error
	TaskID  string
	Message string
}

// UserTaskCompletedData contains data for UserTaskCompleted events.
type UserTaskCompletedData struct {
	Notes string // Optional notes from user
}

// NewEvent creates a new event with the current timestamp.
func NewEvent(eventType EventType, actor string, data interface{}) Event {
	return Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Actor:     actor,
		Data:      data,
	}
}

// NewTaskEvent creates a new event for a specific task.
func NewTaskEvent(eventType EventType, taskID, actor string, data interface{}) Event {
	return Event{
		Type:      eventType,
		Timestamp: time.Now(),
		TaskID:    taskID,
		Actor:     actor,
		Data:      data,
	}
}
```

**Step 2: Verify file compiles**

Run: `go build ./internal/team/...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/team/events.go
git commit -m "feat(team): add event types for TUI integration"
```

---

## Task 2: Add Event Channel to Runner

**Files:**
- Modify: `internal/team/team.go`

**Step 1: Add Events channel to Runner struct**

In `internal/team/team.go`, modify the Runner struct (around line 112-116):

```go
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
```

**Step 2: Add emit helper method**

Add after NewRunner (around line 125):

```go
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
```

**Step 3: Verify file compiles**

Run: `go build ./internal/team/...`
Expected: No errors

**Step 4: Commit**

```bash
git add internal/team/team.go
git commit -m "feat(team): add event channel to Runner for TUI"
```

---

## Task 3: Emit Events in Runner.Run

**Files:**
- Modify: `internal/team/team.go`

**Step 1: Add event emissions in Run method**

Modify the `Run` method to emit events at key points. Update the method (starting around line 127):

```go
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
```

**Step 2: Add events field to ModeExecutor**

In `internal/team/modes.go`, update ModeExecutor struct (around line 13):

```go
// ModeExecutor executes work based on the selected mode.
type ModeExecutor struct {
	registry *provider.Registry
	config   *config.Config
	session  *Session
	events   chan Event // Event channel for TUI
}
```

**Step 3: Add emit helper to ModeExecutor**

Add after the struct definition:

```go
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
```

**Step 4: Verify build**

Run: `go build ./internal/team/...`
Expected: No errors

**Step 5: Commit**

```bash
git add internal/team/team.go internal/team/modes.go
git commit -m "feat(team): emit events throughout workflow"
```

---

## Task 4: Create Kanban Card Model

**Files:**
- Create: `internal/tui/card.go`

**Step 1: Create the card model**

```go
// Package tui provides the Bubble Tea TUI for The Council of Legends.
package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Column represents a Kanban column.
type Column int

const (
	ColumnBacklog Column = iota
	ColumnInProgress
	ColumnReview
	ColumnDone
)

func (c Column) String() string {
	switch c {
	case ColumnBacklog:
		return "Backlog"
	case ColumnInProgress:
		return "In Progress"
	case ColumnReview:
		return "Review"
	case ColumnDone:
		return "Done"
	default:
		return "Unknown"
	}
}

// KanbanCard represents a task card on the board.
type KanbanCard struct {
	ID          string
	Title       string
	Description string
	AssignedTo  string   // AI ID or "user"
	Column      Column
	IsBlocking  bool     // Only relevant for user tasks
	IsUserTask  bool     // True if assigned to human
	StreamBuf   strings.Builder
	FullHistory strings.Builder
	Progress    float64  // 0.0-1.0
	DependsOn   []string
	BlockedBy   []string // Computed: tasks that block this one
	Done        bool
	Error       error
}

// NewKanbanCard creates a new Kanban card.
func NewKanbanCard(id, title, assignedTo string) *KanbanCard {
	return &KanbanCard{
		ID:         id,
		Title:      title,
		AssignedTo: assignedTo,
		Column:     ColumnBacklog,
		IsUserTask: assignedTo == "user",
	}
}

// IsBlocked returns true if this card is blocked by incomplete dependencies.
func (c *KanbanCard) IsBlocked() bool {
	return len(c.BlockedBy) > 0
}

// CardStyle returns the appropriate style for this card's state.
func (c *KanbanCard) CardStyle(styles Styles, selected bool, width int) lipgloss.Style {
	baseStyle := styles.Panel.Width(width - 2)

	// Border color based on state
	var borderColor lipgloss.Color
	switch {
	case selected:
		borderColor = colorBorderActive
	case c.Error != nil:
		borderColor = colorError
	case c.IsBlocked():
		borderColor = colorError
	case c.IsUserTask && c.Column == ColumnInProgress:
		borderColor = colorWarning // Amber for "waiting on you"
	case c.Column == ColumnInProgress:
		borderColor = GetAIColor(c.AssignedTo)
	case c.Column == ColumnReview:
		borderColor = colorPrimary
	case c.Column == ColumnDone:
		borderColor = colorSuccess
	default:
		borderColor = colorBorder
	}

	return baseStyle.BorderForeground(borderColor)
}

// Render renders the card content.
func (c *KanbanCard) Render(styles Styles, selected bool, width, maxLines int) string {
	style := c.CardStyle(styles, selected, width)

	// Header: title
	title := truncateString(c.Title, width-4)

	// Assignee line
	var assignee string
	if c.IsUserTask {
		assignee = styles.Warning.Render("You")
	} else {
		assignee = styles.AINameStyle(c.AssignedTo).Render(c.AssignedTo)
	}

	// Status indicator
	var status string
	switch {
	case c.Error != nil:
		status = styles.Error.Render("Error")
	case c.IsBlocked():
		status = styles.Error.Render("Blocked")
	case c.Column == ColumnInProgress && c.IsUserTask:
		status = styles.Warning.Render("Waiting on you")
	case c.Column == ColumnInProgress:
		status = renderProgressBar(c.Progress, width-4)
	case c.Column == ColumnDone:
		status = styles.Success.Render("Done")
	}

	// Stream preview (truncated)
	preview := ""
	if c.Column == ColumnInProgress && c.StreamBuf.Len() > 0 {
		lines := strings.Split(c.StreamBuf.String(), "\n")
		previewLines := maxLines - 4 // Leave room for title, assignee, status
		if previewLines > 0 && len(lines) > 0 {
			if len(lines) > previewLines {
				lines = lines[len(lines)-previewLines:]
			}
			preview = strings.Join(lines, "\n")
			preview = truncateLines(preview, width-4)
		}
	}

	// Build card content
	var content strings.Builder
	content.WriteString(title)
	content.WriteString("\n")
	content.WriteString(assignee)
	if status != "" {
		content.WriteString("\n")
		content.WriteString(status)
	}
	if preview != "" {
		content.WriteString("\n")
		content.WriteString(styles.Muted.Render(preview))
	}

	return style.Render(content.String())
}

func renderProgressBar(progress float64, width int) string {
	barWidth := width - 2
	if barWidth < 5 {
		barWidth = 5
	}
	filled := int(progress * float64(barWidth))
	empty := barWidth - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return bar
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func truncateLines(s string, maxWidth int) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if len(line) > maxWidth {
			lines[i] = line[:maxWidth-3] + "..."
		}
	}
	return strings.Join(lines, "\n")
}
```

**Step 2: Verify build**

Run: `go build ./internal/tui/...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/tui/card.go
git commit -m "feat(tui): add Kanban card model"
```

---

## Task 5: Create Kanban Board Model

**Files:**
- Create: `internal/tui/kanban.go`

**Step 1: Create the Kanban board model**

```go
// Package tui provides the Bubble Tea TUI for The Council of Legends.
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/thecounciloflegends/council/internal/team"
)

// KanbanModel is the Bubble Tea model for the Kanban board.
type KanbanModel struct {
	// Configuration
	task     string
	styles   Styles

	// State
	pm           string
	pmPhase      team.Phase
	workMode     team.WorkMode
	planSummary  string
	paused       bool

	// Board
	cards        map[string]*KanbanCard
	columns      map[Column][]*KanbanCard
	selectedCol  Column
	selectedRow  int

	// Active panel
	activeCardID string

	// UI
	width        int
	height       int
	ready        bool
	spinner      spinner.Model
	startTime    time.Time

	// Popup
	showPopup    bool
	popupCardID  string
	popupScroll  int

	// Help
	showHelp     bool

	// Events
	events       chan team.Event

	// Completion
	complete     bool
	quitting     bool
}

// NewKanbanModel creates a new Kanban board model.
func NewKanbanModel(task string, events chan team.Event) KanbanModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colorSecondary)

	return KanbanModel{
		task:        task,
		styles:      DefaultStyles(),
		cards:       make(map[string]*KanbanCard),
		columns:     make(map[Column][]*KanbanCard),
		selectedCol: ColumnBacklog,
		spinner:     s,
		startTime:   time.Now(),
		events:      events,
	}
}

// TeamEventMsg wraps a team event for Bubble Tea.
type TeamEventMsg struct {
	Event team.Event
}

// Init initializes the model.
func (m KanbanModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tea.EnterAltScreen,
		m.listenForEvents(),
	)
}

// listenForEvents returns a command that listens for team events.
func (m KanbanModel) listenForEvents() tea.Cmd {
	return func() tea.Msg {
		event, ok := <-m.events
		if !ok {
			return nil
		}
		return TeamEventMsg{Event: event}
	}
}

// Update handles messages.
func (m KanbanModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle popup keys first
		if m.showPopup {
			return m.handlePopupKey(msg)
		}
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case TeamEventMsg:
		m.handleTeamEvent(msg.Event)
		cmds = append(cmds, m.listenForEvents())
	}

	return m, tea.Batch(cmds...)
}

func (m KanbanModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "?":
		m.showHelp = !m.showHelp

	case "h", "left":
		if m.selectedCol > ColumnBacklog {
			m.selectedCol--
			m.selectedRow = 0
		}

	case "l", "right":
		if m.selectedCol < ColumnDone {
			m.selectedCol++
			m.selectedRow = 0
		}

	case "j", "down":
		cards := m.columns[m.selectedCol]
		if m.selectedRow < len(cards)-1 {
			m.selectedRow++
		}

	case "k", "up":
		if m.selectedRow > 0 {
			m.selectedRow--
		}

	case "enter":
		// Open popup for selected card
		if card := m.getSelectedCard(); card != nil {
			m.showPopup = true
			m.popupCardID = card.ID
			m.popupScroll = 0
		}

	case "tab":
		// Cycle active card focus
		m.cycleActiveCard()

	case " ":
		// Toggle pause
		m.paused = !m.paused

	case "d":
		// Mark user task done (quick)
		if card := m.getSelectedCard(); card != nil && card.IsUserTask {
			m.completeUserTask(card.ID, "")
		}

	case "D":
		// Mark user task done with notes (TODO: open form)
		if card := m.getSelectedCard(); card != nil && card.IsUserTask {
			m.completeUserTask(card.ID, "")
		}

	case "r":
		// Reassign (only when paused)
		if m.paused {
			// TODO: open reassign form
		}

	case "a":
		// Add task (only when paused)
		if m.paused {
			// TODO: open add task form
		}
	}

	return m, nil
}

func (m KanbanModel) handlePopupKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.showPopup = false
	case "j", "down":
		m.popupScroll++
	case "k", "up":
		if m.popupScroll > 0 {
			m.popupScroll--
		}
	}
	return m, nil
}

func (m *KanbanModel) handleTeamEvent(event team.Event) {
	switch event.Type {
	case team.EventPhaseChanged:
		if data, ok := event.Data.(team.PhaseChangedData); ok {
			m.pmPhase = data.NewPhase
		}

	case team.EventPMDecision:
		if data, ok := event.Data.(team.PMDecisionData); ok {
			m.workMode = data.WorkMode
			m.planSummary = data.PlanSummary
		}

	case team.EventTaskCreated:
		if data, ok := event.Data.(team.TaskCreatedData); ok {
			card := NewKanbanCard(event.TaskID, data.Title, data.AssignedTo)
			card.Description = data.Description
			card.DependsOn = data.DependsOn
			card.IsBlocking = data.IsBlocking
			card.IsUserTask = data.AssignedTo == "user"
			m.cards[event.TaskID] = card
			m.columns[ColumnBacklog] = append(m.columns[ColumnBacklog], card)
		}

	case team.EventTaskStarted:
		if card, ok := m.cards[event.TaskID]; ok {
			m.moveCard(card, ColumnInProgress)
			m.activeCardID = card.ID
		}

	case team.EventTaskProgress:
		if card, ok := m.cards[event.TaskID]; ok {
			if data, ok := event.Data.(team.TaskProgressData); ok {
				card.StreamBuf.WriteString(data.Content)
				card.FullHistory.WriteString(data.Content)
				card.Progress = data.Progress
			}
		}

	case team.EventTaskCompleted:
		if card, ok := m.cards[event.TaskID]; ok {
			card.Done = true
			m.moveCard(card, ColumnReview)
		}

	case team.EventTaskMovedToReview:
		if card, ok := m.cards[event.TaskID]; ok {
			m.moveCard(card, ColumnReview)
		}

	case team.EventPMApproved:
		if card, ok := m.cards[event.TaskID]; ok {
			m.moveCard(card, ColumnDone)
		}

	case team.EventUserTaskCreated:
		if data, ok := event.Data.(team.TaskCreatedData); ok {
			card := NewKanbanCard(event.TaskID, data.Title, "user")
			card.Description = data.Description
			card.DependsOn = data.DependsOn
			card.IsBlocking = data.IsBlocking
			card.IsUserTask = true
			m.cards[event.TaskID] = card
			m.columns[ColumnBacklog] = append(m.columns[ColumnBacklog], card)
		}

	case team.EventSessionComplete:
		m.complete = true

	case team.EventError:
		if data, ok := event.Data.(team.ErrorData); ok {
			if card, ok := m.cards[data.TaskID]; ok {
				card.Error = data.Error
			}
		}
	}
}

func (m *KanbanModel) moveCard(card *KanbanCard, toColumn Column) {
	// Remove from old column
	oldCol := m.columns[card.Column]
	for i, c := range oldCol {
		if c.ID == card.ID {
			m.columns[card.Column] = append(oldCol[:i], oldCol[i+1:]...)
			break
		}
	}

	// Add to new column
	card.Column = toColumn
	m.columns[toColumn] = append(m.columns[toColumn], card)
}

func (m *KanbanModel) getSelectedCard() *KanbanCard {
	cards := m.columns[m.selectedCol]
	if m.selectedRow >= 0 && m.selectedRow < len(cards) {
		return cards[m.selectedRow]
	}
	return nil
}

func (m *KanbanModel) cycleActiveCard() {
	// Find next in-progress card
	inProgress := m.columns[ColumnInProgress]
	if len(inProgress) == 0 {
		return
	}

	currentIdx := -1
	for i, card := range inProgress {
		if card.ID == m.activeCardID {
			currentIdx = i
			break
		}
	}

	nextIdx := (currentIdx + 1) % len(inProgress)
	m.activeCardID = inProgress[nextIdx].ID
}

func (m *KanbanModel) completeUserTask(taskID, notes string) {
	if card, ok := m.cards[taskID]; ok && card.IsUserTask {
		card.Done = true
		m.moveCard(card, ColumnDone)
		// TODO: emit event back to team runner
	}
}

// View renders the model.
func (m KanbanModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.quitting {
		return "Goodbye!\n"
	}

	var b strings.Builder

	// Render popup if open
	if m.showPopup {
		return m.renderPopup()
	}

	// Render help if open
	if m.showHelp {
		return m.renderHelp()
	}

	// Header
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// PM Section
	b.WriteString(m.renderPMSection())
	b.WriteString("\n")

	// Main content: Board + Active Panel
	b.WriteString(m.renderMainContent())

	// Help bar
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar())

	return b.String()
}

func (m KanbanModel) renderHeader() string {
	title := m.styles.Title.Render("THE COUNCIL OF LEGENDS - Team Mode")

	// Status indicator
	status := "Running"
	statusStyle := m.styles.Success
	if m.paused {
		status = "Paused"
		statusStyle = m.styles.Warning
	} else if m.complete {
		status = "Complete"
	}

	phase := fmt.Sprintf("Phase: %s", m.pmPhase)

	header := lipgloss.JoinHorizontal(lipgloss.Center,
		title,
		strings.Repeat(" ", m.width-lipgloss.Width(title)-lipgloss.Width(status)-lipgloss.Width(phase)-10),
		m.styles.Muted.Render(phase),
		"  ",
		statusStyle.Render(status),
	)

	return m.styles.Header.Width(m.width).Render(header)
}

func (m KanbanModel) renderPMSection() string {
	pmName := m.pm
	if pmName == "" {
		pmName = "Selecting..."
	}

	pmLine := fmt.Sprintf("PM: %s", m.styles.AINameStyle(m.pm).Render(pmName))
	taskLine := fmt.Sprintf("Task: %s", truncateString(m.task, m.width-20))

	var modeLine string
	if m.workMode != "" {
		// Count tasks
		total := len(m.cards)
		done := len(m.columns[ColumnDone])
		modeLine = fmt.Sprintf("Mode: %s | Progress: %d/%d tasks", m.workMode, done, total)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, pmLine, taskLine, modeLine)

	return m.styles.Panel.Width(m.width - 2).Render(content)
}

func (m KanbanModel) renderMainContent() string {
	// Calculate dimensions
	boardWidth := int(float64(m.width) * 0.7)
	panelWidth := m.width - boardWidth - 2
	contentHeight := m.height - 12 // Leave room for header, PM, help

	// Render board
	board := m.renderBoard(boardWidth, contentHeight)

	// Render active panel
	panel := m.renderActivePanel(panelWidth, contentHeight)

	return lipgloss.JoinHorizontal(lipgloss.Top, board, panel)
}

func (m KanbanModel) renderBoard(width, height int) string {
	colWidth := (width - 4) / 4

	var columns []string
	for col := ColumnBacklog; col <= ColumnDone; col++ {
		columns = append(columns, m.renderColumn(col, colWidth, height))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, columns...)
}

func (m KanbanModel) renderColumn(col Column, width, height int) string {
	// Column header
	cards := m.columns[col]
	headerText := fmt.Sprintf("%s (%d)", col.String(), len(cards))

	headerStyle := m.styles.PanelHeader.Width(width - 2)
	if col == m.selectedCol {
		headerStyle = headerStyle.Foreground(colorPrimary)
	}
	header := headerStyle.Render(headerText)

	// Cards
	cardHeight := 6
	maxCards := (height - 4) / cardHeight
	if maxCards < 1 {
		maxCards = 1
	}

	var cardViews []string
	for i, card := range cards {
		if i >= maxCards {
			remaining := len(cards) - maxCards
			cardViews = append(cardViews, m.styles.Muted.Render(fmt.Sprintf("+%d more", remaining)))
			break
		}

		selected := col == m.selectedCol && i == m.selectedRow
		cardViews = append(cardViews, card.Render(m.styles, selected, width-2, cardHeight-1))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, cardViews...)

	colStyle := m.styles.Panel.Width(width - 1).Height(height)
	return colStyle.Render(lipgloss.JoinVertical(lipgloss.Left, header, content))
}

func (m KanbanModel) renderActivePanel(width, height int) string {
	header := m.styles.PanelHeader.Width(width - 2).Render("Active")

	var content string
	if m.activeCardID != "" {
		if card, ok := m.cards[m.activeCardID]; ok {
			aiName := m.styles.AINameStyle(card.AssignedTo).Render(card.AssignedTo)
			content = lipgloss.JoinVertical(lipgloss.Left,
				aiName,
				m.styles.Muted.Render(strings.Repeat("─", width-4)),
				card.StreamBuf.String(),
			)
		}
	} else {
		content = m.styles.Muted.Render("No active task")
	}

	panelStyle := m.styles.Panel.Width(width - 1).Height(height)
	return panelStyle.Render(lipgloss.JoinVertical(lipgloss.Left, header, content))
}

func (m KanbanModel) renderPopup() string {
	card, ok := m.cards[m.popupCardID]
	if !ok {
		return ""
	}

	popupWidth := m.width - 10
	popupHeight := m.height - 6

	// Title bar
	title := fmt.Sprintf("  %s  ", card.Title)
	closeHint := "[X]"
	titleBar := lipgloss.JoinHorizontal(lipgloss.Center,
		m.styles.Title.Render(title),
		strings.Repeat(" ", popupWidth-lipgloss.Width(title)-lipgloss.Width(closeHint)-4),
		m.styles.Muted.Render(closeHint),
	)

	// Metadata
	assignee := fmt.Sprintf("Assigned: %s", card.AssignedTo)
	status := fmt.Sprintf("Status: %s", card.Column.String())
	meta := lipgloss.JoinHorizontal(lipgloss.Left, assignee, "    ", status)

	// Dependencies
	var deps string
	if len(card.DependsOn) > 0 {
		deps = fmt.Sprintf("Depends on: %s", strings.Join(card.DependsOn, ", "))
	}

	// Description
	desc := m.styles.Label.Render("Description:") + "\n" + card.Description

	// Activity log
	activity := m.styles.Label.Render("Activity Log:") + "\n" + card.FullHistory.String()

	// Help bar
	helpBar := m.styles.HelpBar.Render("[Esc] Close   [j/k] Scroll")

	content := lipgloss.JoinVertical(lipgloss.Left,
		titleBar,
		m.styles.Muted.Render(strings.Repeat("─", popupWidth-2)),
		meta,
		deps,
		m.styles.Muted.Render(strings.Repeat("─", popupWidth-2)),
		desc,
		m.styles.Muted.Render(strings.Repeat("─", popupWidth-2)),
		activity,
	)

	popupStyle := m.styles.PanelFocused.
		Width(popupWidth).
		Height(popupHeight).
		Align(lipgloss.Left)

	popup := popupStyle.Render(content)

	// Center on screen
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Left, popup, helpBar))
}

func (m KanbanModel) renderHelp() string {
	help := `
KEYBOARD CONTROLS

Navigation
  h/l or ←/→    Move between columns
  j/k or ↑/↓    Move between cards
  Tab           Cycle active panel focus
  Enter         Open card details

Control
  Space         Pause/Resume
  q             Quit

User Tasks
  d             Mark done (quick)
  D             Mark done with notes

Intervention (paused only)
  r             Reassign task
  a             Add new task

Press any key to close
`

	helpStyle := m.styles.Panel.
		Width(50).
		Padding(2)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		helpStyle.Render(help))
}

func (m KanbanModel) renderHelpBar() string {
	var help string
	if m.paused {
		help = "[←→↑↓] Navigate  [Enter] Details  [Space] Resume  [r] Reassign  [a] Add  [?] Help  [q] Quit"
	} else {
		help = "[←→↑↓] Navigate  [Enter] Details  [Space] Pause  [d] Done  [Tab] Focus  [?] Help  [q] Quit"
	}
	return m.styles.HelpBar.Width(m.width).Render(help)
}
```

**Step 2: Verify build**

Run: `go build ./internal/tui/...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/tui/kanban.go
git commit -m "feat(tui): add Kanban board model"
```

---

## Task 6: Create Team TUI Runner

**Files:**
- Create: `internal/tui/teamtui.go`

**Step 1: Create the team TUI runner**

```go
// Package tui provides the Bubble Tea TUI for The Council of Legends.
package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/thecounciloflegends/council/internal/config"
	"github.com/thecounciloflegends/council/internal/provider"
	"github.com/thecounciloflegends/council/internal/team"
)

// TeamTUIOptions configures the team TUI.
type TeamTUIOptions struct {
	Task    string
	PM      string
	Mode    team.WorkMode
	Members []string
}

// RunTeamTUI runs the team collaboration with TUI.
func RunTeamTUI(ctx context.Context, opts TeamTUIOptions, cfg *config.Config, registry *provider.Registry) error {
	// Create team runner
	runner := team.NewRunner(registry, cfg)

	// Create Kanban model with events channel
	model := NewKanbanModel(opts.Task, runner.Events)

	// Create Bubble Tea program
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Run team in background
	teamOpts := team.Options{
		Task:            opts.Task,
		PM:              opts.PM,
		Mode:            opts.Mode,
		Members:         opts.Members,
		CheckpointLevel: team.CheckpointNone, // TUI handles checkpoints
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- runner.Run(ctx, teamOpts)
		close(runner.Events)
	}()

	// Run TUI
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// Check for team errors
	select {
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("team error: %w", err)
		}
	default:
	}

	return nil
}
```

**Step 2: Verify build**

Run: `go build ./internal/tui/...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/tui/teamtui.go
git commit -m "feat(tui): add team TUI runner"
```

---

## Task 7: Add --tui Flag to Team CLI

**Files:**
- Modify: `cmd/team/main.go`

**Step 1: Add tui flag and import**

At the top of `cmd/team/main.go`, add the import:

```go
import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/thecounciloflegends/council/internal/config"
	"github.com/thecounciloflegends/council/internal/provider"
	"github.com/thecounciloflegends/council/internal/team"
	"github.com/thecounciloflegends/council/internal/tui"
)
```

Add the flag variable (around line 25):

```go
var (
	cfgFile         string
	pm              string
	workMode        string
	members         []string
	includeArbiter  bool
	checkpointLevel string
	showCosts       bool
	outputDir       string
	verbose         bool
	useTUI          bool  // NEW
)
```

In `init()`, add the flag (around line 67):

```go
	rootCmd.Flags().BoolVar(&useTUI, "tui", false, "use interactive TUI with Kanban board")
```

**Step 2: Update runTeam to handle TUI mode**

Replace the `runTeam` function:

```go
func runTeam(cmd *cobra.Command, args []string) error {
	task := args[0]

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Set default members if not specified
	teamMembers := members
	if len(teamMembers) == 0 {
		teamMembers = cfg.GetCouncilMembers()
	}

	// Add arbiter if requested
	if includeArbiter {
		teamMembers = append(teamMembers, "groq")
	}

	registry := setupProviders(cfg)

	// TUI mode
	if useTUI {
		var mode team.WorkMode
		switch workMode {
		case "pair_programming":
			mode = team.ModePairProgramming
		case "consultation":
			mode = team.ModeConsultation
		case "round_robin":
			mode = team.ModeRoundRobin
		case "divide_conquer":
			mode = team.ModeDivideConquer
		case "free_form":
			mode = team.ModeFreeForm
		}

		tuiOpts := tui.TeamTUIOptions{
			Task:    task,
			PM:      pm,
			Mode:    mode,
			Members: teamMembers,
		}

		return tui.RunTeamTUI(cmd.Context(), tuiOpts, cfg, registry)
	}

	// Console mode (existing code)
	// Parse checkpoint level
	cpLevel := team.CheckpointAll
	switch checkpointLevel {
	case "major":
		cpLevel = team.CheckpointMajor
	case "none":
		cpLevel = team.CheckpointNone
	}

	// Parse work mode
	var mode team.WorkMode
	switch workMode {
	case "pair_programming":
		mode = team.ModePairProgramming
	case "consultation":
		mode = team.ModeConsultation
	case "round_robin":
		mode = team.ModeRoundRobin
	case "divide_conquer":
		mode = team.ModeDivideConquer
	case "free_form":
		mode = team.ModeFreeForm
	}

	// Create output directory if specified
	if outputDir == "" {
		// Use default projects directory
		outputDir = filepath.Join(cfg.Team.ProjectsDir, fmt.Sprintf("team_%d", os.Getpid()))
	}

	opts := team.Options{
		Task:            task,
		PM:              pm,
		Mode:            mode,
		Members:         teamMembers,
		IncludeArbiter:  includeArbiter,
		CheckpointLevel: cpLevel,
		ShowCosts:       showCosts,
		OutputDir:       outputDir,
		Verbose:         verbose,
	}

	runner := team.NewRunner(registry, cfg)
	return runner.Run(cmd.Context(), opts)
}
```

**Step 3: Verify build**

Run: `go build ./cmd/team`
Expected: No errors

**Step 4: Commit**

```bash
git add cmd/team/main.go
git commit -m "feat(team): add --tui flag for Kanban board"
```

---

## Task 8: Add Event Emissions in ModeExecutor

**Files:**
- Modify: `internal/team/modes.go`

**Step 1: Update executeDivideConquer to emit events**

This is the most representative mode. Update it to emit task events (around line 285):

```go
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
```

**Step 2: Verify build**

Run: `go build ./internal/team/...`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/team/modes.go
git commit -m "feat(team): emit events in divide_conquer mode"
```

---

## Task 9: Test Build and Basic Functionality

**Step 1: Build all binaries**

Run: `go build ./...`
Expected: No errors

**Step 2: Test help output**

Run: `./team --help`
Expected: Shows --tui flag in help

**Step 3: Commit final state**

```bash
git add -A
git commit -m "feat: Team TUI with Kanban board - complete implementation"
```

---

## Task 10: Push and Verify

**Step 1: Push to remote**

Run: `git push origin the-council-of-many`

**Step 2: Final verification**

Run: `./team "Test task" --tui`
Expected: TUI launches with Kanban board (may error on API calls without keys, but UI should render)

---

## Summary

This plan implements:
1. Event types for TUI communication
2. Event channel in team Runner
3. Event emissions throughout workflow
4. Kanban card model with styling
5. Kanban board Bubble Tea model
6. Team TUI runner orchestration
7. --tui flag in team CLI
8. Streaming event emissions in divide_conquer mode

Total: ~800 lines of new Go code across 5 new files and 3 modified files.
