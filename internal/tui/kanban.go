// Package tui provides the Bubble Tea TUI for The Council of Legends.
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jxmullins/thekanbansociety/internal/team"
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

	// Activity status
	activityStatus string
	lastActivity   time.Time

	// Debug log
	showDebugLog bool
	debugLog     []DebugLogEntry
	debugScroll  int

	// Completion
	complete     bool
	quitting     bool
}

// DebugLogEntry represents a single debug log entry.
type DebugLogEntry struct {
	Timestamp time.Time
	Type      string // "event", "cmd", "response", "error"
	Actor     string
	Message   string
}

// NewKanbanModel creates a new Kanban board model.
func NewKanbanModel(task string, events chan team.Event) KanbanModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colorSecondary)

	// Pre-initialize all columns to avoid nil map entries
	columns := make(map[Column][]*KanbanCard)
	columns[ColumnBacklog] = make([]*KanbanCard, 0)
	columns[ColumnInProgress] = make([]*KanbanCard, 0)
	columns[ColumnReview] = make([]*KanbanCard, 0)
	columns[ColumnDone] = make([]*KanbanCard, 0)

	return KanbanModel{
		task:           task,
		styles:         DefaultStyles(),
		cards:          make(map[string]*KanbanCard),
		columns:        columns,
		selectedCol:    ColumnBacklog,
		spinner:        s,
		startTime:      time.Now(),
		events:         events,
		activityStatus: "Initializing...",
		lastActivity:   time.Now(),
		debugLog:       make([]DebugLogEntry, 0),
	}
}

// addDebugLog adds an entry to the debug log.
func (m *KanbanModel) addDebugLog(logType, actor, message string) {
	entry := DebugLogEntry{
		Timestamp: time.Now(),
		Type:      logType,
		Actor:     actor,
		Message:   message,
	}
	m.debugLog = append(m.debugLog, entry)
	// Keep only last 100 entries
	if len(m.debugLog) > 100 {
		m.debugLog = m.debugLog[len(m.debugLog)-100:]
	}
	m.lastActivity = time.Now()
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
		if m.showDebugLog {
			return m.handleDebugLogKey(msg)
		}
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		// Force a clear and redraw on resize
		cmds = append(cmds, tea.ClearScreen)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case TeamEventMsg:
		m = *m.handleTeamEventCopy(msg.Event)
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

	case "`", "~":
		m.showDebugLog = !m.showDebugLog
		m.debugScroll = len(m.debugLog) - 1 // Scroll to bottom

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
		if cards != nil && len(cards) > 0 && m.selectedRow < len(cards)-1 {
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

func (m KanbanModel) handleDebugLogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "`", "~":
		m.showDebugLog = false
	case "j", "down":
		if m.debugScroll < len(m.debugLog)-1 {
			m.debugScroll++
		}
	case "k", "up":
		if m.debugScroll > 0 {
			m.debugScroll--
		}
	case "g":
		m.debugScroll = 0 // Go to top
	case "G":
		m.debugScroll = len(m.debugLog) - 1 // Go to bottom
	}
	return m, nil
}

func (m KanbanModel) handleTeamEventCopy(event team.Event) *KanbanModel {
	// Log all events
	m.addDebugLog("event", event.Actor, fmt.Sprintf("[%s] %s", event.Type.String(), event.TaskID))
	m.lastActivity = time.Now()

	switch event.Type {
	case team.EventPhaseChanged:
		if data, ok := event.Data.(team.PhaseChangedData); ok {
			m.pmPhase = data.NewPhase
			m.activityStatus = fmt.Sprintf("Phase: %s", data.NewPhase.String())
			m.addDebugLog("phase", "system", fmt.Sprintf("Phase changed: %s → %s", data.OldPhase.String(), data.NewPhase.String()))
		}

	case team.EventPMSelected:
		if data, ok := event.Data.(team.PMSelectedData); ok {
			m.pm = data.PMID
			m.activityStatus = fmt.Sprintf("PM selected: %s", data.PMID)
			m.addDebugLog("pm", data.PMID, "Selected as Project Manager")
		}

	case team.EventPMDecision:
		m.pm = event.Actor
		if data, ok := event.Data.(team.PMDecisionData); ok {
			m.workMode = data.WorkMode
			m.planSummary = data.PlanSummary
			m.activityStatus = fmt.Sprintf("Planning: %s mode", data.WorkMode)
			m.addDebugLog("decision", event.Actor, fmt.Sprintf("Work mode: %s, Tasks: %d", data.WorkMode, len(data.Tasks)))
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
			m.activityStatus = fmt.Sprintf("Task created: %s", truncateString(data.Title, 30))
			m.addDebugLog("task", data.AssignedTo, fmt.Sprintf("Created: %s", data.Title))
		}

	case team.EventTaskStarted:
		if card, ok := m.cards[event.TaskID]; ok {
			m.moveCard(card, ColumnInProgress)
			m.activeCardID = card.ID
			m.activityStatus = fmt.Sprintf("%s working on: %s", event.Actor, truncateString(card.Title, 20))
			m.addDebugLog("cmd", event.Actor, fmt.Sprintf("Started: %s", card.Title))
		}

	case team.EventTaskProgress:
		if card, ok := m.cards[event.TaskID]; ok {
			if data, ok := event.Data.(team.TaskProgressData); ok {
				card.StreamBuf.WriteString(data.Content)
				card.FullHistory.WriteString(data.Content)
				card.Progress = data.Progress
				// Only log significant progress updates
				if len(data.Content) > 50 {
					m.addDebugLog("response", event.Actor, truncateString(data.Content, 80))
				}
			}
		}
		m.activityStatus = fmt.Sprintf("%s responding...", event.Actor)

	case team.EventTaskCompleted:
		if card, ok := m.cards[event.TaskID]; ok {
			card.Done = true
			m.moveCard(card, ColumnReview)
			m.activityStatus = fmt.Sprintf("%s completed: %s", event.Actor, truncateString(card.Title, 20))
			m.addDebugLog("task", event.Actor, fmt.Sprintf("Completed: %s", card.Title))
		}

	case team.EventTaskMovedToReview:
		if card, ok := m.cards[event.TaskID]; ok {
			m.moveCard(card, ColumnReview)
			m.activityStatus = fmt.Sprintf("Review: %s", truncateString(card.Title, 25))
			m.addDebugLog("review", "pm", fmt.Sprintf("Moved to review: %s", card.Title))
		}

	case team.EventPMApproved:
		if card, ok := m.cards[event.TaskID]; ok {
			m.moveCard(card, ColumnDone)
			m.activityStatus = fmt.Sprintf("Approved: %s", truncateString(card.Title, 25))
			m.addDebugLog("approved", "pm", fmt.Sprintf("Approved: %s", card.Title))
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
			m.activityStatus = fmt.Sprintf("Your task: %s", truncateString(data.Title, 25))
			m.addDebugLog("user", "pm", fmt.Sprintf("Assigned to you: %s", data.Title))
		}

	case team.EventSessionComplete:
		m.complete = true
		m.activityStatus = "Session complete!"
		m.addDebugLog("complete", "system", "Session finished successfully")

	case team.EventError:
		if data, ok := event.Data.(team.ErrorData); ok {
			if card, ok := m.cards[data.TaskID]; ok {
				card.Error = data.Error
			}
			m.activityStatus = fmt.Sprintf("Error: %s", truncateString(data.Message, 30))
			m.addDebugLog("error", event.Actor, data.Message)
		}
	}
	return &m
}

func (m *KanbanModel) moveCard(card *KanbanCard, toColumn Column) {
	if card == nil {
		return
	}

	// Remove from old column
	oldCol := m.columns[card.Column]
	newOldCol := make([]*KanbanCard, 0, len(oldCol))
	for _, c := range oldCol {
		if c != nil && c.ID != card.ID {
			newOldCol = append(newOldCol, c)
		}
	}
	m.columns[card.Column] = newOldCol

	// Add to new column
	card.Column = toColumn
	if m.columns[toColumn] == nil {
		m.columns[toColumn] = make([]*KanbanCard, 0)
	}
	m.columns[toColumn] = append(m.columns[toColumn], card)
}

func (m *KanbanModel) getSelectedCard() *KanbanCard {
	cards := m.columns[m.selectedCol]
	if cards == nil || len(cards) == 0 {
		return nil
	}
	if m.selectedRow >= 0 && m.selectedRow < len(cards) {
		card := cards[m.selectedRow]
		if card != nil {
			return card
		}
	}
	// Try to return the first valid card if selectedRow is invalid
	for _, card := range cards {
		if card != nil {
			return card
		}
	}
	return nil
}

func (m *KanbanModel) cycleActiveCard() {
	// Find next in-progress card
	inProgress := m.columns[ColumnInProgress]
	if len(inProgress) == 0 {
		m.activeCardID = ""
		return
	}

	currentIdx := -1
	for i, card := range inProgress {
		if card != nil && card.ID == m.activeCardID {
			currentIdx = i
			break
		}
	}

	nextIdx := (currentIdx + 1) % len(inProgress)
	if nextIdx >= 0 && nextIdx < len(inProgress) && inProgress[nextIdx] != nil {
		m.activeCardID = inProgress[nextIdx].ID
	} else if len(inProgress) > 0 && inProgress[0] != nil {
		m.activeCardID = inProgress[0].ID
	} else {
		m.activeCardID = ""
	}
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

	// Handle too-small window
	if m.width < 60 || m.height < 15 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			m.styles.Warning.Render("Window too small\nMinimum: 60x15"))
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

	// Render debug log if open
	if m.showDebugLog {
		return m.renderDebugLog()
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

	// Status indicator with spinner
	var status string
	statusStyle := m.styles.Success
	if m.paused {
		status = "⏸ Paused"
		statusStyle = m.styles.Warning
	} else if m.complete {
		status = "✓ Complete"
	} else {
		// Show spinner when active
		status = m.spinner.View() + " Running"
	}

	phase := fmt.Sprintf("Phase: %s", m.pmPhase)

	header := lipgloss.JoinHorizontal(lipgloss.Center,
		title,
		strings.Repeat(" ", max(0, m.width-lipgloss.Width(title)-lipgloss.Width(status)-lipgloss.Width(phase)-10)),
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
		total := len(m.cards)
		done := len(m.columns[ColumnDone])
		modeLine = fmt.Sprintf("Mode: %s | Progress: %d/%d tasks", m.workMode, done, total)
	}

	// Activity status line with elapsed time
	elapsed := time.Since(m.lastActivity).Round(time.Second)
	activityLine := m.styles.Muted.Render(fmt.Sprintf("Activity: %s (%s ago)", m.activityStatus, elapsed))

	content := lipgloss.JoinVertical(lipgloss.Left, pmLine, taskLine, modeLine, activityLine)

	return m.styles.Panel.Width(m.width - 2).Render(content)
}

func (m KanbanModel) renderMainContent() string {
	// Calculate dimensions with safeguards
	boardWidth := int(float64(m.width) * 0.7)
	if boardWidth < 40 {
		boardWidth = 40
	}
	panelWidth := m.width - boardWidth - 2
	if panelWidth < 20 {
		panelWidth = 20
	}
	contentHeight := m.height - 12 // Leave room for header, PM, help
	if contentHeight < 10 {
		contentHeight = 10
	}

	// Render board
	board := m.renderBoard(boardWidth, contentHeight)

	// Render active panel
	panel := m.renderActivePanel(panelWidth, contentHeight)

	return lipgloss.JoinHorizontal(lipgloss.Top, board, panel)
}

func (m KanbanModel) renderBoard(width, height int) string {
	// Ensure minimum dimensions
	if width < 20 {
		width = 20
	}
	if height < 10 {
		height = 10
	}

	colWidth := (width - 4) / 4
	if colWidth < 10 {
		colWidth = 10
	}

	var columns []string
	for col := ColumnBacklog; col <= ColumnDone; col++ {
		columns = append(columns, m.renderColumn(col, colWidth, height))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, columns...)
}

func (m KanbanModel) renderColumn(col Column, width, height int) string {
	// Ensure minimum dimensions
	if width < 10 {
		width = 10
	}
	if height < 5 {
		height = 5
	}

	// Column header - safely get card count
	cards := m.columns[col]
	cardCount := 0
	for _, c := range cards {
		if c != nil {
			cardCount++
		}
	}
	headerText := fmt.Sprintf("%s (%d)", col.String(), cardCount)

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
	validCardIdx := 0
	for i, card := range cards {
		if card == nil {
			continue // Skip nil cards
		}
		if validCardIdx >= maxCards {
			remaining := cardCount - maxCards
			if remaining > 0 {
				cardViews = append(cardViews, m.styles.Muted.Render(fmt.Sprintf("+%d more", remaining)))
			}
			break
		}

		selected := col == m.selectedCol && i == m.selectedRow
		cardView := card.Render(m.styles, selected, width-2, cardHeight-1)
		cardViews = append(cardViews, cardView)
		validCardIdx++
	}

	var content string
	if len(cardViews) > 0 {
		content = lipgloss.JoinVertical(lipgloss.Left, cardViews...)
	}

	colStyle := m.styles.Panel.Width(width - 1).Height(height)
	return colStyle.Render(lipgloss.JoinVertical(lipgloss.Left, header, content))
}

func (m KanbanModel) renderActivePanel(width, height int) string {
	// Ensure minimum dimensions
	if width < 15 {
		width = 15
	}
	if height < 5 {
		height = 5
	}

	header := m.styles.PanelHeader.Width(width - 2).Render("Active")

	var content string
	if m.activeCardID != "" {
		if card, ok := m.cards[m.activeCardID]; ok && card != nil {
			assignedTo := card.AssignedTo
			if assignedTo == "" {
				assignedTo = "unknown"
			}
			aiName := m.styles.AINameStyle(assignedTo).Render(assignedTo)
			dividerWidth := width - 4
			if dividerWidth < 5 {
				dividerWidth = 5
			}
			streamContent := card.StreamBuf.String()
			// Truncate stream content to fit panel
			maxStreamLines := height - 6
			if maxStreamLines > 0 {
				lines := strings.Split(streamContent, "\n")
				if len(lines) > maxStreamLines {
					lines = lines[len(lines)-maxStreamLines:]
				}
				streamContent = strings.Join(lines, "\n")
			}
			content = lipgloss.JoinVertical(lipgloss.Left,
				aiName,
				m.styles.Muted.Render(strings.Repeat("─", dividerWidth)),
				streamContent,
			)
		} else {
			content = m.styles.Muted.Render("No active task")
		}
	} else {
		content = m.styles.Muted.Render("No active task")
	}

	panelStyle := m.styles.Panel.Width(width - 1).Height(height)
	return panelStyle.Render(lipgloss.JoinVertical(lipgloss.Left, header, content))
}

func (m KanbanModel) renderPopup() string {
	card, ok := m.cards[m.popupCardID]
	if !ok || card == nil {
		m.showPopup = false
		return m.styles.Muted.Render("Card not found")
	}

	popupWidth := m.width - 10
	if popupWidth < 40 {
		popupWidth = 40
	}
	popupHeight := m.height - 6
	if popupHeight < 20 {
		popupHeight = 20
	}

	// Title bar
	cardTitle := card.Title
	if cardTitle == "" {
		cardTitle = "Untitled"
	}
	title := fmt.Sprintf("  %s  ", truncateString(cardTitle, popupWidth-20))
	closeHint := "[X]"
	titleWidth := lipgloss.Width(title)
	closeWidth := lipgloss.Width(closeHint)
	spacerWidth := popupWidth - titleWidth - closeWidth - 4
	if spacerWidth < 1 {
		spacerWidth = 1
	}
	titleBar := lipgloss.JoinHorizontal(lipgloss.Center,
		m.styles.Title.Render(title),
		strings.Repeat(" ", spacerWidth),
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
	description := card.Description
	if description == "" {
		description = "(no description)"
	}
	desc := m.styles.Label.Render("Description:") + "\n" + description

	// Activity log
	activityContent := card.FullHistory.String()
	if activityContent == "" {
		activityContent = "(no activity yet)"
	}
	activity := m.styles.Label.Render("Activity Log:") + "\n" + activityContent

	// Help bar
	helpBar := m.styles.HelpBar.Render("[Esc] Close   [j/k] Scroll")

	dividerWidth := popupWidth - 2
	if dividerWidth < 10 {
		dividerWidth = 10
	}
	divider := m.styles.Muted.Render(strings.Repeat("─", dividerWidth))

	content := lipgloss.JoinVertical(lipgloss.Left,
		titleBar,
		divider,
		meta,
		deps,
		divider,
		desc,
		divider,
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

func (m KanbanModel) renderDebugLog() string {
	var b strings.Builder

	title := m.styles.Title.Render("Debug Log")
	hint := m.styles.Muted.Render("Press ` or ~ to close | j/k to scroll")

	b.WriteString(title)
	b.WriteString("  ")
	b.WriteString(hint)
	b.WriteString("\n")
	b.WriteString(m.styles.Muted.Render(strings.Repeat("─", m.width-4)))
	b.WriteString("\n\n")

	if len(m.debugLog) == 0 {
		b.WriteString(m.styles.Muted.Render("No activity yet..."))
	} else {
		// Calculate visible range
		maxVisible := m.height - 8
		if maxVisible < 5 {
			maxVisible = 5
		}

		start := 0
		if len(m.debugLog) > maxVisible {
			start = len(m.debugLog) - maxVisible
			if m.debugScroll >= 0 && m.debugScroll < len(m.debugLog)-maxVisible {
				start = m.debugScroll
			}
		}
		end := start + maxVisible
		if end > len(m.debugLog) {
			end = len(m.debugLog)
		}

		for i := start; i < end; i++ {
			entry := m.debugLog[i]
			timestamp := entry.Timestamp.Format("15:04:05")

			// Color based on type
			var typeStyle lipgloss.Style
			switch entry.Type {
			case "event":
				typeStyle = m.styles.Muted
			case "phase":
				typeStyle = m.styles.Success
			case "pm", "decision":
				typeStyle = m.styles.AINameStyle(entry.Actor)
			case "task", "cmd":
				typeStyle = m.styles.AINameStyle(entry.Actor)
			case "response":
				typeStyle = m.styles.Subtitle
			case "error":
				typeStyle = m.styles.Error
			case "complete":
				typeStyle = m.styles.Success
			default:
				typeStyle = m.styles.Muted
			}

			line := fmt.Sprintf("[%s] %s %s: %s",
				m.styles.Muted.Render(timestamp),
				typeStyle.Render(fmt.Sprintf("%-8s", entry.Type)),
				m.styles.AINameStyle(entry.Actor).Render(entry.Actor),
				entry.Message,
			)
			b.WriteString(line)
			b.WriteString("\n")
		}

		// Scroll indicator
		if len(m.debugLog) > maxVisible {
			scrollInfo := fmt.Sprintf("\n[%d-%d of %d entries]", start+1, end, len(m.debugLog))
			b.WriteString(m.styles.Muted.Render(scrollInfo))
		}
	}

	logStyle := m.styles.Panel.
		Width(m.width - 4).
		Height(m.height - 4).
		Padding(1)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		logStyle.Render(b.String()))
}

func (m KanbanModel) renderHelpBar() string {
	var help string
	if m.paused {
		help = "[←→↑↓] Navigate  [Enter] Details  [Space] Resume  [r] Reassign  [`] Log  [?] Help  [q] Quit"
	} else {
		help = "[←→↑↓] Navigate  [Enter] Details  [Space] Pause  [d] Done  [`] Log  [?] Help  [q] Quit"
	}
	return m.styles.HelpBar.Width(m.width).Render(help)
}
