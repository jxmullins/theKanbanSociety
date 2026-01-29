// Package persona provides dynamic persona management for AI participants.
package persona

import (
	"context"
	"fmt"
	"strings"

	"github.com/jxmullins/thekanbansociety/internal/config"
	"github.com/jxmullins/thekanbansociety/internal/provider"
)

// Persona represents an AI's debate persona.
type Persona struct {
	ID           string
	Name         string
	Description  string
	SystemPrompt string
	Traits       []string
	Style        string
}

// PersonaSwitch represents a persona change event.
type PersonaSwitch struct {
	AIID        string
	FromPersona string
	ToPersona   string
	Reason      string
	Round       int
}

// DynamicPersonaManager manages persona switching during debates.
type DynamicPersonaManager struct {
	registry    *provider.Registry
	config      *config.Config
	personas    map[string]*Persona
	assignments map[string]string // AIID -> current persona ID
	history     []PersonaSwitch
	enabled     bool
}

// NewDynamicPersonaManager creates a new persona manager.
func NewDynamicPersonaManager(registry *provider.Registry, cfg *config.Config) *DynamicPersonaManager {
	return &DynamicPersonaManager{
		registry:    registry,
		config:      cfg,
		personas:    make(map[string]*Persona),
		assignments: make(map[string]string),
		history:     []PersonaSwitch{},
		enabled:     true,
	}
}

// RegisterPersona adds a persona to the manager.
func (m *DynamicPersonaManager) RegisterPersona(p *Persona) {
	m.personas[p.ID] = p
}

// GetDefaultPersonas returns the built-in personas.
func GetDefaultPersonas() []*Persona {
	return []*Persona{
		{
			ID:          "analytical",
			Name:        "Analytical Debater",
			Description: "Focuses on logical analysis and evidence",
			SystemPrompt: `You are an analytical debater who prioritizes:
- Logical reasoning and evidence-based arguments
- Identifying assumptions and testing them
- Structured analysis of pros and cons
- Quantitative data when available`,
			Traits: []string{"logical", "evidence-based", "systematic"},
			Style:  "analytical",
		},
		{
			ID:          "devils_advocate",
			Name:        "Devil's Advocate",
			Description: "Challenges assumptions and popular positions",
			SystemPrompt: `You are a devil's advocate who:
- Questions commonly held assumptions
- Presents counterarguments to prevailing views
- Highlights potential flaws in popular positions
- Ensures all perspectives are thoroughly examined`,
			Traits: []string{"contrarian", "challenging", "thorough"},
			Style:  "adversarial",
		},
		{
			ID:          "synthesizer",
			Name:        "Synthesizer",
			Description: "Finds common ground and integrates perspectives",
			SystemPrompt: `You are a synthesizer who:
- Identifies areas of agreement among different positions
- Builds bridges between opposing views
- Proposes integrative solutions
- Focuses on constructive outcomes`,
			Traits: []string{"integrative", "diplomatic", "constructive"},
			Style:  "collaborative",
		},
		{
			ID:          "pragmatist",
			Name:        "Pragmatist",
			Description: "Focuses on practical implementation",
			SystemPrompt: `You are a pragmatist who:
- Evaluates ideas based on practical feasibility
- Considers real-world constraints and trade-offs
- Focuses on actionable recommendations
- Prioritizes what works over what's ideal`,
			Traits: []string{"practical", "realistic", "action-oriented"},
			Style:  "practical",
		},
		{
			ID:          "visionary",
			Name:        "Visionary",
			Description: "Explores bold possibilities and future implications",
			SystemPrompt: `You are a visionary who:
- Thinks beyond current constraints
- Explores long-term implications
- Proposes innovative solutions
- Challenges the status quo`,
			Traits: []string{"innovative", "forward-thinking", "bold"},
			Style:  "visionary",
		},
	}
}

// AssignPersona assigns a persona to an AI.
func (m *DynamicPersonaManager) AssignPersona(aiID, personaID string) {
	m.assignments[aiID] = personaID
}

// GetPersona returns the current persona for an AI.
func (m *DynamicPersonaManager) GetPersona(aiID string) *Persona {
	personaID, ok := m.assignments[aiID]
	if !ok {
		return nil
	}
	return m.personas[personaID]
}

// GetSystemPrompt returns the system prompt for an AI based on its persona.
func (m *DynamicPersonaManager) GetSystemPrompt(aiID string) string {
	persona := m.GetPersona(aiID)
	if persona == nil {
		return ""
	}
	return persona.SystemPrompt
}

// SuggestSwitch analyzes debate context and suggests persona switches.
func (m *DynamicPersonaManager) SuggestSwitch(ctx context.Context, round int, topic string, responses []string) ([]PersonaSwitch, error) {
	if !m.enabled {
		return nil, nil
	}

	// Analyze debate state
	prompt := fmt.Sprintf(`Analyze this debate state and suggest if any participant should shift their approach.

Topic: %s
Round: %d

Recent responses:
%s

For each participant that should change approach, respond with:
AI: [ai_id]
FROM: [current_style]
TO: [suggested_persona: analytical, devils_advocate, synthesizer, pragmatist, visionary]
REASON: [brief reason]

If no changes needed, respond: NO_CHANGES_NEEDED`,
		topic, round, strings.Join(responses, "\n---\n"))

	resp, err := m.registry.Invoke(ctx, "claude", provider.Request{
		Prompt:       prompt,
		SystemPrompt: "You are a debate moderator analyzing participant dynamics.",
	})
	if err != nil {
		return nil, err
	}

	// Parse suggestions
	return m.parseSuggestions(resp.Content, round), nil
}

func (m *DynamicPersonaManager) parseSuggestions(content string, round int) []PersonaSwitch {
	if strings.Contains(content, "NO_CHANGES_NEEDED") {
		return nil
	}

	var switches []PersonaSwitch
	lines := strings.Split(content, "\n")

	var current PersonaSwitch
	current.Round = round

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "AI:") {
			if current.AIID != "" && current.ToPersona != "" {
				switches = append(switches, current)
			}
			current = PersonaSwitch{Round: round}
			current.AIID = strings.TrimSpace(strings.TrimPrefix(line, "AI:"))
		} else if strings.HasPrefix(line, "FROM:") {
			current.FromPersona = strings.TrimSpace(strings.TrimPrefix(line, "FROM:"))
		} else if strings.HasPrefix(line, "TO:") {
			current.ToPersona = strings.TrimSpace(strings.TrimPrefix(line, "TO:"))
		} else if strings.HasPrefix(line, "REASON:") {
			current.Reason = strings.TrimSpace(strings.TrimPrefix(line, "REASON:"))
		}
	}

	// Add last switch if valid
	if current.AIID != "" && current.ToPersona != "" {
		switches = append(switches, current)
	}

	return switches
}

// ApplySwitch applies a persona switch.
func (m *DynamicPersonaManager) ApplySwitch(sw PersonaSwitch) {
	m.assignments[sw.AIID] = sw.ToPersona
	m.history = append(m.history, sw)
}

// GetHistory returns the persona switch history.
func (m *DynamicPersonaManager) GetHistory() []PersonaSwitch {
	return m.history
}

// Enable enables dynamic persona switching.
func (m *DynamicPersonaManager) Enable() {
	m.enabled = true
}

// Disable disables dynamic persona switching.
func (m *DynamicPersonaManager) Disable() {
	m.enabled = false
}

// IsEnabled returns whether dynamic switching is enabled.
func (m *DynamicPersonaManager) IsEnabled() bool {
	return m.enabled
}

// LoadPersonasFromConfig loads personas from config files.
func (m *DynamicPersonaManager) LoadPersonasFromConfig(personaDir string) error {
	personas, err := config.LoadPersonasFromDir(personaDir)
	if err != nil {
		return err
	}

	for id, p := range personas {
		m.RegisterPersona(&Persona{
			ID:           id,
			Name:         p.Name,
			Description:  p.Description,
			SystemPrompt: p.SystemPrompt,
			Traits:       p.Traits,
			Style:        p.DebateStyle,
		})
	}

	return nil
}

// InitializeDefaults loads the default personas.
func (m *DynamicPersonaManager) InitializeDefaults() {
	for _, p := range GetDefaultPersonas() {
		m.RegisterPersona(p)
	}
}
