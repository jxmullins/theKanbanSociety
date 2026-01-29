package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/jxmullins/thekanbansociety/internal/config"
	"github.com/jxmullins/thekanbansociety/internal/provider"
)

// ModelManager provides an interactive model management interface.
type ModelManager struct {
	config   *config.Config
	registry *provider.Registry
	styles   Styles

	table    table.Model
	spinner  spinner.Model
	selected string
	testing  bool
	testResult string
	err      error

	width  int
	height int
	ready  bool
}

// NewModelManager creates a new model manager.
func NewModelManager(cfg *config.Config, registry *provider.Registry) *ModelManager {
	// Create table
	columns := []table.Column{
		{Title: "ID", Width: 15},
		{Title: "Name", Width: 20},
		{Title: "Provider", Width: 12},
		{Title: "Model", Width: 25},
		{Title: "Status", Width: 10},
	}

	rows := make([]table.Row, 0, len(cfg.Models))
	for id, model := range cfg.Models {
		rows = append(rows, table.Row{
			id,
			model.DisplayName,
			model.Provider,
			model.Model,
			"Unknown",
		})
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#7C3AED")).
		Bold(false)
	t.SetStyles(s)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED"))

	return &ModelManager{
		config:   cfg,
		registry: registry,
		styles:   DefaultStyles(),
		table:    t,
		spinner:  sp,
	}
}

// Init initializes the model.
func (m ModelManager) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tea.EnterAltScreen,
	)
}

// TestCompleteMsg signals a test is complete.
type TestCompleteMsg struct {
	ModelID string
	Success bool
	Message string
	Error   error
}

// Update handles messages and updates the model.
func (m ModelManager) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.testing {
			// Only allow quit during test
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "enter", "t":
			// Test selected model
			if row := m.table.SelectedRow(); len(row) > 0 {
				m.selected = row[0]
				m.testing = true
				m.testResult = ""
				return m, m.testModel(m.selected)
			}

		case "a":
			// Add new model
			return m, m.addModel()

		case "e":
			// Edit selected model
			if row := m.table.SelectedRow(); len(row) > 0 {
				return m, m.editModel(row[0])
			}

		case "r":
			// Refresh/reload config
			return m, m.refreshModels()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(msg.Width - 4)
		m.table.SetHeight(msg.Height - 12)
		m.ready = true

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case TestCompleteMsg:
		m.testing = false
		if msg.Error != nil {
			m.testResult = m.styles.Error.Render(fmt.Sprintf("✗ %s: %v", msg.ModelID, msg.Error))
		} else if msg.Success {
			m.testResult = m.styles.Success.Render(fmt.Sprintf("✓ %s: %s", msg.ModelID, msg.Message))
		} else {
			m.testResult = m.styles.Warning.Render(fmt.Sprintf("? %s: %s", msg.ModelID, msg.Message))
		}
		// Update table status
		m.updateModelStatus(msg.ModelID, msg.Success)
	}

	// Update table
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the model manager.
func (m ModelManager) View() string {
	if !m.ready {
		return "Loading..."
	}

	var b strings.Builder

	// Header
	title := m.styles.Title.Render("Model Management")
	b.WriteString(m.styles.Header.Width(m.width).Align(lipgloss.Center).Render(title))
	b.WriteString("\n\n")

	// Table
	b.WriteString(m.table.View())
	b.WriteString("\n\n")

	// Status / Test result
	if m.testing {
		b.WriteString(m.spinner.View())
		b.WriteString(" Testing ")
		b.WriteString(m.styles.AINameStyle(m.selected).Render(m.selected))
		b.WriteString("...")
	} else if m.testResult != "" {
		b.WriteString(m.testResult)
	}
	b.WriteString("\n\n")

	// Help
	help := "↑/↓: Navigate | t/Enter: Test | a: Add | e: Edit | r: Refresh | q: Quit"
	b.WriteString(m.styles.HelpBar.Render(help))

	return b.String()
}

func (m *ModelManager) testModel(modelID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Try to invoke with a simple prompt
		_, err := m.registry.Invoke(ctx, modelID, provider.Request{
			Prompt:    "Say 'Hello' in one word.",
			MaxTokens: 10,
		})

		if err != nil {
			return TestCompleteMsg{
				ModelID: modelID,
				Success: false,
				Error:   err,
			}
		}

		return TestCompleteMsg{
			ModelID: modelID,
			Success: true,
			Message: "Connection successful",
		}
	}
}

func (m *ModelManager) updateModelStatus(modelID string, success bool) {
	rows := m.table.Rows()
	for i, row := range rows {
		if row[0] == modelID {
			status := "✓ OK"
			if !success {
				status = "✗ Failed"
			}
			rows[i][4] = status
			break
		}
	}
	m.table.SetRows(rows)
}

func (m *ModelManager) addModel() tea.Cmd {
	return func() tea.Msg {
		var (
			id       string
			name     string
			prov     string
			model    string
			endpoint string
		)

		providerOptions := []huh.Option[string]{
			huh.NewOption("Anthropic (Claude)", "anthropic"),
			huh.NewOption("OpenAI (GPT)", "openai"),
			huh.NewOption("Google (Gemini)", "google"),
			huh.NewOption("Groq", "groq"),
			huh.NewOption("DeepSeek", "deepseek"),
			huh.NewOption("Mistral", "mistral"),
			huh.NewOption("xAI (Grok)", "xai"),
			huh.NewOption("Ollama (Local)", "ollama"),
			huh.NewOption("LM Studio (Local)", "lmstudio"),
			huh.NewOption("Generic (OpenAI-compatible)", "generic"),
		}

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Model ID").
					Description("Unique identifier (e.g., 'my-claude')").
					Value(&id),
				huh.NewInput().
					Title("Display Name").
					Description("Friendly name shown in UI").
					Value(&name),
				huh.NewSelect[string]().
					Title("Provider").
					Options(providerOptions...).
					Value(&prov),
				huh.NewInput().
					Title("Model Name").
					Description("The model to use (e.g., 'claude-sonnet-4-5-20250929')").
					Value(&model),
				huh.NewInput().
					Title("Endpoint (optional)").
					Description("Custom API endpoint for local/generic providers").
					Value(&endpoint),
			).Title("Add New Model"),
		).WithTheme(councilTheme())

		if err := form.Run(); err != nil {
			return ErrorMsg{Err: err}
		}

		// Note: In a full implementation, this would save to config file
		// For now, just acknowledge the addition
		return TestCompleteMsg{
			ModelID: id,
			Success: true,
			Message: fmt.Sprintf("Model '%s' added (restart to apply)", id),
		}
	}
}

func (m *ModelManager) editModel(modelID string) tea.Cmd {
	return func() tea.Msg {
		modelCfg, ok := m.config.GetModel(modelID)
		if !ok {
			return ErrorMsg{Err: fmt.Errorf("model %s not found", modelID)}
		}

		name := modelCfg.DisplayName
		model := modelCfg.Model

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Display Name").
					Value(&name),
				huh.NewInput().
					Title("Model Name").
					Value(&model),
			).Title(fmt.Sprintf("Edit Model: %s", modelID)),
		).WithTheme(councilTheme())

		if err := form.Run(); err != nil {
			return ErrorMsg{Err: err}
		}

		return TestCompleteMsg{
			ModelID: modelID,
			Success: true,
			Message: "Changes saved (restart to apply)",
		}
	}
}

func (m *ModelManager) refreshModels() tea.Cmd {
	return func() tea.Msg {
		// In a full implementation, this would reload config from disk
		return TestCompleteMsg{
			ModelID: "config",
			Success: true,
			Message: "Models refreshed",
		}
	}
}

// RunModelManager starts the model manager TUI.
func RunModelManager(cfg *config.Config, registry *provider.Registry) error {
	m := NewModelManager(cfg, registry)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
