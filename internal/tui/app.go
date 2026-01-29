// Package tui provides the Bubble Tea TUI for The Council of Legends.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jxmullins/thekanbansociety/internal/config"
	"github.com/jxmullins/thekanbansociety/internal/provider"
)

// ViewMode represents the current view mode.
type ViewMode int

const (
	ViewGrid ViewMode = iota
	ViewFocus
	ViewTranscript
)

// Phase represents the current debate phase.
type Phase int

const (
	PhaseSetup Phase = iota
	PhaseOpening
	PhaseRebuttal
	PhaseSynthesis
	PhaseFinal
	PhaseComplete
)

func (p Phase) String() string {
	switch p {
	case PhaseSetup:
		return "Setup"
	case PhaseOpening:
		return "Opening Statements"
	case PhaseRebuttal:
		return "Rebuttals"
	case PhaseSynthesis:
		return "Synthesis"
	case PhaseFinal:
		return "Final Verdict"
	case PhaseComplete:
		return "Complete"
	default:
		return "Unknown"
	}
}

// Options configures the TUI app.
type Options struct {
	Topic     string
	Mode      string
	Rounds    int
	Members   []string
	OutputDir string
}

// Model is the main Bubble Tea model.
type Model struct {
	// Configuration
	options  Options
	config   *config.Config
	registry *provider.Registry
	styles   Styles

	// State
	viewMode     ViewMode
	phase        Phase
	round        int
	focusedPanel int
	width        int
	height       int
	ready        bool
	quitting     bool
	err          error

	// Panels
	panels []Panel

	// UI components
	spinner spinner.Model

	// Timing
	startTime time.Time
	phaseTime time.Time

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// Panel represents a single AI's panel in the grid view.
type Panel struct {
	AIID        string
	DisplayName string
	Content     strings.Builder
	Streaming   bool
	Done        bool
	Error       error
	ScrollPos   int
}

// NewModel creates a new TUI model.
func NewModel(opts Options, cfg *config.Config, registry *provider.Registry) Model {
	ctx, cancel := context.WithCancel(context.Background())

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED"))

	// Create panels for each member
	panels := make([]Panel, len(opts.Members))
	for i, member := range opts.Members {
		displayName := member
		if modelCfg, ok := cfg.GetModel(member); ok && modelCfg.DisplayName != "" {
			displayName = modelCfg.DisplayName
		}
		panels[i] = Panel{
			AIID:        member,
			DisplayName: displayName,
		}
	}

	return Model{
		options:      opts,
		config:       cfg,
		registry:     registry,
		styles:       DefaultStyles(),
		viewMode:     ViewGrid,
		phase:        PhaseSetup,
		round:        0,
		focusedPanel: 0,
		panels:       panels,
		spinner:      s,
		startTime:    time.Now(),
		phaseTime:    time.Now(),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tea.EnterAltScreen,
		m.startDebate(),
	)
}

// StreamChunkMsg carries a streaming chunk from an AI.
type StreamChunkMsg struct {
	AIID    string
	Content string
	Done    bool
	Error   error
}

// PhaseCompleteMsg signals a phase is complete.
type PhaseCompleteMsg struct {
	Phase Phase
}

// DebateCompleteMsg signals the debate is complete.
type DebateCompleteMsg struct{}

// ErrorMsg carries an error.
type ErrorMsg struct {
	Err error
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			m.cancel()
			return m, tea.Quit

		case "tab":
			// Cycle through panels
			m.focusedPanel = (m.focusedPanel + 1) % len(m.panels)

		case "shift+tab":
			// Cycle backwards through panels
			m.focusedPanel = (m.focusedPanel - 1 + len(m.panels)) % len(m.panels)

		case "h", "left":
			if m.focusedPanel > 0 {
				m.focusedPanel--
			}

		case "l", "right":
			if m.focusedPanel < len(m.panels)-1 {
				m.focusedPanel++
			}

		case "j", "down":
			// Scroll down in focused panel
			m.panels[m.focusedPanel].ScrollPos++

		case "k", "up":
			// Scroll up in focused panel
			if m.panels[m.focusedPanel].ScrollPos > 0 {
				m.panels[m.focusedPanel].ScrollPos--
			}

		case "enter", "f":
			// Toggle focus mode
			if m.viewMode == ViewFocus {
				m.viewMode = ViewGrid
			} else {
				m.viewMode = ViewFocus
			}

		case "t":
			// Toggle transcript mode
			if m.viewMode == ViewTranscript {
				m.viewMode = ViewGrid
			} else {
				m.viewMode = ViewTranscript
			}

		case "g":
			// Return to grid view
			m.viewMode = ViewGrid
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case StreamChunkMsg:
		// Update the appropriate panel
		for i := range m.panels {
			if m.panels[i].AIID == msg.AIID {
				if msg.Error != nil {
					m.panels[i].Error = msg.Error
					m.panels[i].Streaming = false
					m.panels[i].Done = true
				} else if msg.Done {
					m.panels[i].Streaming = false
					m.panels[i].Done = true
				} else {
					m.panels[i].Content.WriteString(msg.Content)
				}
				break
			}
		}

	case PhaseCompleteMsg:
		m.phaseTime = time.Now()
		// Move to next phase
		switch msg.Phase {
		case PhaseOpening:
			if m.round < m.options.Rounds {
				m.phase = PhaseRebuttal
				m.round++
				cmds = append(cmds, m.runRebuttalRound())
			} else {
				m.phase = PhaseSynthesis
				cmds = append(cmds, m.runSynthesisRound())
			}
		case PhaseRebuttal:
			if m.round < m.options.Rounds {
				m.round++
				cmds = append(cmds, m.runRebuttalRound())
			} else {
				m.phase = PhaseSynthesis
				cmds = append(cmds, m.runSynthesisRound())
			}
		case PhaseSynthesis:
			m.phase = PhaseFinal
			cmds = append(cmds, m.runFinalRound())
		case PhaseFinal:
			m.phase = PhaseComplete
		}

	case DebateCompleteMsg:
		m.phase = PhaseComplete

	case ErrorMsg:
		m.err = msg.Err
	}

	return m, tea.Batch(cmds...)
}

// View renders the model.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.quitting {
		return "Goodbye!\n"
	}

	var b strings.Builder

	// Header
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// Main content based on view mode
	switch m.viewMode {
	case ViewFocus:
		b.WriteString(m.renderFocusView())
	case ViewTranscript:
		b.WriteString(m.renderTranscriptView())
	default:
		b.WriteString(m.renderGridView())
	}

	// Footer / Status bar
	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar())

	return b.String()
}

func (m Model) renderHeader() string {
	title := m.styles.Title.Render("THE COUNCIL OF LEGENDS")
	topic := m.styles.Subtitle.Render(m.options.Topic)

	header := lipgloss.JoinVertical(lipgloss.Center, title, topic)
	return m.styles.Header.Width(m.width).Align(lipgloss.Center).Render(header)
}

func (m Model) renderGridView() string {
	if len(m.panels) == 0 {
		return "No panels"
	}

	// Calculate panel dimensions
	panelWidth := (m.width - 4) / min(len(m.panels), 3)
	panelHeight := m.height - 10

	var rows []string
	var currentRow []string

	for i, panel := range m.panels {
		panelView := m.renderPanel(panel, i == m.focusedPanel, panelWidth, panelHeight)
		currentRow = append(currentRow, panelView)

		// 3 panels per row
		if len(currentRow) == 3 || i == len(m.panels)-1 {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, currentRow...))
			currentRow = nil
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m Model) renderPanel(panel Panel, focused bool, width, height int) string {
	style := m.styles.PanelStyle(focused)
	if focused {
		style = style.BorderForeground(GetAIColor(panel.AIID))
	}
	style = style.Width(width - 2).Height(height)

	// Header with AI name and status
	nameStyle := m.styles.AINameStyle(panel.AIID)
	header := nameStyle.Render(panel.DisplayName)

	if panel.Streaming {
		header += " " + m.spinner.View()
	} else if panel.Done {
		header += " " + m.styles.Success.Render("✓")
	}
	if panel.Error != nil {
		header += " " + m.styles.Error.Render("✗")
	}

	// Content
	content := panel.Content.String()
	if content == "" && panel.Streaming {
		content = m.styles.Muted.Render("Waiting for response...")
	} else if panel.Error != nil {
		content = m.styles.Error.Render(panel.Error.Error())
	}

	// Truncate/scroll content
	lines := strings.Split(content, "\n")
	maxLines := height - 4
	if maxLines < 1 {
		maxLines = 1
	}

	start := panel.ScrollPos
	if start > len(lines)-maxLines {
		start = max(0, len(lines)-maxLines)
	}
	end := min(start+maxLines, len(lines))

	visibleContent := strings.Join(lines[start:end], "\n")

	panelContent := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		visibleContent,
	)

	return style.Render(panelContent)
}

func (m Model) renderFocusView() string {
	if m.focusedPanel >= len(m.panels) {
		return ""
	}

	panel := m.panels[m.focusedPanel]
	style := m.styles.PanelFocused.
		Width(m.width - 4).
		Height(m.height - 10).
		BorderForeground(GetAIColor(panel.AIID))

	nameStyle := m.styles.AINameStyle(panel.AIID)
	header := nameStyle.Render(panel.DisplayName)

	if panel.Streaming {
		header += " " + m.spinner.View()
	}

	content := panel.Content.String()
	if content == "" {
		content = m.styles.Muted.Render("No content yet...")
	}

	return style.Render(lipgloss.JoinVertical(lipgloss.Left, header, "", content))
}

func (m Model) renderTranscriptView() string {
	var b strings.Builder

	b.WriteString(m.styles.Title.Render("Full Transcript"))
	b.WriteString("\n\n")

	for _, panel := range m.panels {
		nameStyle := m.styles.AINameStyle(panel.AIID)
		b.WriteString(nameStyle.Render(panel.DisplayName))
		b.WriteString("\n")
		b.WriteString(strings.Repeat("─", 40))
		b.WriteString("\n")
		b.WriteString(panel.Content.String())
		b.WriteString("\n\n")
	}

	return b.String()
}

func (m Model) renderStatusBar() string {
	elapsed := time.Since(m.startTime).Round(time.Second)

	left := fmt.Sprintf(" %s | Round %d/%d | %s",
		m.phase.String(),
		max(m.round, 1),
		m.options.Rounds,
		m.options.Mode,
	)

	right := fmt.Sprintf("%s ", elapsed)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}

	return m.styles.StatusBar.Width(m.width).Render(
		left + strings.Repeat(" ", gap) + right,
	)
}

func (m Model) renderHelpBar() string {
	help := "q: quit | tab: next panel | hjkl: navigate | f: focus | t: transcript | g: grid"
	return m.styles.HelpBar.Render(help)
}

// startDebate initiates the debate sequence.
func (m Model) startDebate() tea.Cmd {
	return func() tea.Msg {
		// Start with opening round
		m.phase = PhaseOpening
		m.round = 1
		return nil
	}
}

// runOpeningRound runs the opening statements phase.
func (m Model) runOpeningRound() tea.Cmd {
	return func() tea.Msg {
		// This will be implemented to stream responses
		return PhaseCompleteMsg{Phase: PhaseOpening}
	}
}

// runRebuttalRound runs a rebuttal round.
func (m Model) runRebuttalRound() tea.Cmd {
	return func() tea.Msg {
		return PhaseCompleteMsg{Phase: PhaseRebuttal}
	}
}

// runSynthesisRound runs the synthesis phase.
func (m Model) runSynthesisRound() tea.Cmd {
	return func() tea.Msg {
		return PhaseCompleteMsg{Phase: PhaseSynthesis}
	}
}

// runFinalRound runs the final verdict phase.
func (m Model) runFinalRound() tea.Cmd {
	return func() tea.Msg {
		return PhaseCompleteMsg{Phase: PhaseFinal}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
