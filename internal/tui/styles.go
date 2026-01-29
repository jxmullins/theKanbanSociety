package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette
var (
	// Primary colors
	colorPrimary   = lipgloss.Color("#7C3AED") // Purple
	colorSecondary = lipgloss.Color("#06B6D4") // Cyan
	colorAccent    = lipgloss.Color("#F59E0B") // Amber

	// AI-specific colors
	colorClaude  = lipgloss.Color("#D97706") // Orange (Anthropic)
	colorGPT     = lipgloss.Color("#10B981") // Green (OpenAI)
	colorGemini  = lipgloss.Color("#3B82F6") // Blue (Google)
	colorGroq    = lipgloss.Color("#8B5CF6") // Purple
	colorMistral = lipgloss.Color("#EC4899") // Pink
	colorGrok    = lipgloss.Color("#EF4444") // Red
	colorOllama  = lipgloss.Color("#6B7280") // Gray
	colorDefault = lipgloss.Color("#9CA3AF") // Light gray

	// UI colors
	colorBorder       = lipgloss.Color("#4B5563")
	colorBorderActive = lipgloss.Color("#7C3AED")
	colorText         = lipgloss.Color("#F3F4F6")
	colorTextMuted    = lipgloss.Color("#9CA3AF")
	colorBackground   = lipgloss.Color("#1F2937")
	colorSuccess      = lipgloss.Color("#10B981")
	colorWarning      = lipgloss.Color("#F59E0B")
	colorError        = lipgloss.Color("#EF4444")
)

// GetAIColor returns the color associated with an AI provider.
func GetAIColor(aiID string) lipgloss.Color {
	switch aiID {
	case "claude", "claude-opus":
		return colorClaude
	case "gpt", "o3", "o4-mini":
		return colorGPT
	case "gemini", "gemini-flash":
		return colorGemini
	case "groq":
		return colorGroq
	case "mistral":
		return colorMistral
	case "grok":
		return colorGrok
	case "ollama", "lmstudio":
		return colorOllama
	default:
		return colorDefault
	}
}

// Styles holds all the application styles.
type Styles struct {
	// App-level styles
	App        lipgloss.Style
	Header     lipgloss.Style
	Footer     lipgloss.Style
	StatusBar  lipgloss.Style
	HelpBar    lipgloss.Style

	// Panel styles
	Panel         lipgloss.Style
	PanelActive   lipgloss.Style
	PanelHeader   lipgloss.Style
	PanelContent  lipgloss.Style
	PanelFocused  lipgloss.Style

	// Text styles
	Title       lipgloss.Style
	Subtitle    lipgloss.Style
	Label       lipgloss.Style
	Value       lipgloss.Style
	Muted       lipgloss.Style
	Error       lipgloss.Style
	Success     lipgloss.Style
	Warning     lipgloss.Style

	// Content styles
	AIName      lipgloss.Style
	RoundLabel  lipgloss.Style
	Transcript  lipgloss.Style
	Spinner     lipgloss.Style
}

// DefaultStyles returns the default style configuration.
func DefaultStyles() Styles {
	return Styles{
		// App-level styles
		App: lipgloss.NewStyle().
			Background(colorBackground),

		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			BorderStyle(lipgloss.DoubleBorder()).
			BorderBottom(true).
			BorderForeground(colorBorder).
			Padding(0, 1).
			MarginBottom(1),

		Footer: lipgloss.NewStyle().
			Foreground(colorTextMuted).
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(colorBorder).
			Padding(0, 1),

		StatusBar: lipgloss.NewStyle().
			Foreground(colorText).
			Background(lipgloss.Color("#374151")).
			Padding(0, 1),

		HelpBar: lipgloss.NewStyle().
			Foreground(colorTextMuted).
			Padding(0, 1),

		// Panel styles
		Panel: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1),

		PanelActive: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorBorderActive).
			Padding(0, 1),

		PanelHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorText).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(colorBorder).
			MarginBottom(1),

		PanelContent: lipgloss.NewStyle().
			Foreground(colorText),

		PanelFocused: lipgloss.NewStyle().
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 1),

		// Text styles
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary),

		Subtitle: lipgloss.NewStyle().
			Foreground(colorSecondary),

		Label: lipgloss.NewStyle().
			Foreground(colorTextMuted),

		Value: lipgloss.NewStyle().
			Foreground(colorText),

		Muted: lipgloss.NewStyle().
			Foreground(colorTextMuted),

		Error: lipgloss.NewStyle().
			Foreground(colorError),

		Success: lipgloss.NewStyle().
			Foreground(colorSuccess),

		Warning: lipgloss.NewStyle().
			Foreground(colorWarning),

		// Content styles
		AIName: lipgloss.NewStyle().
			Bold(true),

		RoundLabel: lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true),

		Transcript: lipgloss.NewStyle().
			Foreground(colorText),

		Spinner: lipgloss.NewStyle().
			Foreground(colorSecondary),
	}
}

// AINameStyle returns a styled AI name with the appropriate color.
func (s Styles) AINameStyle(aiID string) lipgloss.Style {
	return s.AIName.Foreground(GetAIColor(aiID))
}

// PanelStyle returns the appropriate panel style based on active state.
func (s Styles) PanelStyle(active bool) lipgloss.Style {
	if active {
		return s.PanelActive
	}
	return s.Panel
}
