package tui

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/jxmullins/thekanbansociety/internal/config"
)

// SetupForm holds the form state for debate setup.
type SetupForm struct {
	Topic   string
	Mode    string
	Rounds  int
	Members []string

	config         *config.Config
	availableModels []string
}

// NewSetupForm creates a new setup form with config defaults.
func NewSetupForm(cfg *config.Config) *SetupForm {
	// Get available model IDs
	models := make([]string, 0, len(cfg.Models))
	for id := range cfg.Models {
		models = append(models, id)
	}

	return &SetupForm{
		Topic:           "",
		Mode:            "collaborative",
		Rounds:          cfg.Debate.DefaultRounds,
		Members:         cfg.GetCouncilMembers(),
		config:          cfg,
		availableModels: models,
	}
}

// Run displays the setup form and returns the result.
func (f *SetupForm) Run() error {
	// Build member options with display names
	memberOptions := make([]huh.Option[string], 0, len(f.availableModels))
	for _, id := range f.availableModels {
		displayName := id
		if modelCfg, ok := f.config.GetModel(id); ok && modelCfg.DisplayName != "" {
			displayName = fmt.Sprintf("%s (%s)", modelCfg.DisplayName, modelCfg.Provider)
		}
		memberOptions = append(memberOptions, huh.NewOption(displayName, id))
	}

	// Mode options
	modeOptions := []huh.Option[string]{
		huh.NewOption("Collaborative - AIs work together toward consensus", "collaborative"),
		huh.NewOption("Adversarial - AIs argue opposing positions", "adversarial"),
		huh.NewOption("Socratic - AIs use questioning to explore the topic", "socratic"),
	}

	// Round options
	roundOptions := []huh.Option[int]{
		huh.NewOption("2 rounds (Quick)", 2),
		huh.NewOption("3 rounds (Standard)", 3),
		huh.NewOption("4 rounds (Extended)", 4),
		huh.NewOption("5 rounds (Deep Dive)", 5),
	}

	// Create the form
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Debate Topic").
				Description("What should the council debate?").
				Placeholder("Enter your topic or question...").
				Value(&f.Topic).
				Validate(func(s string) error {
					if len(s) < 5 {
						return fmt.Errorf("topic must be at least 5 characters")
					}
					return nil
				}),
		).Title("The Council of Legends").Description("Configure your debate"),

		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Debate Mode").
				Description("How should the AIs interact?").
				Options(modeOptions...).
				Value(&f.Mode),

			huh.NewSelect[int]().
				Title("Number of Rounds").
				Description("More rounds = deeper discussion").
				Options(roundOptions...).
				Value(&f.Rounds),
		).Title("Debate Settings"),

		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Council Members").
				Description("Select which AIs will participate").
				Options(memberOptions...).
				Value(&f.Members).
				Validate(func(s []string) error {
					if len(s) < 2 {
						return fmt.Errorf("select at least 2 council members")
					}
					return nil
				}),
		).Title("Select Council"),

		huh.NewGroup(
			huh.NewConfirm().
				Title("Ready to Begin?").
				Description(f.getSummary()).
				Affirmative("Start Debate").
				Negative("Cancel"),
		).Title("Confirm"),
	).WithTheme(councilTheme())

	return form.Run()
}

// getSummary returns a summary of the current selections.
func (f *SetupForm) getSummary() string {
	memberCount := len(f.Members)
	if memberCount == 0 {
		memberCount = len(f.config.GetCouncilMembers())
	}
	return fmt.Sprintf("Topic: %s\nMode: %s\nRounds: %d\nMembers: %d AIs",
		truncate(f.Topic, 40),
		f.Mode,
		f.Rounds,
		memberCount,
	)
}

// GetOptions returns the form values as TUI Options.
func (f *SetupForm) GetOptions(outputDir string) Options {
	return Options{
		Topic:     f.Topic,
		Mode:      f.Mode,
		Rounds:    f.Rounds,
		Members:   f.Members,
		OutputDir: outputDir,
	}
}

// councilTheme returns a custom Huh theme for the council.
func councilTheme() *huh.Theme {
	t := huh.ThemeDracula()

	// Customize colors
	t.Focused.Title = t.Focused.Title.Foreground(lipgloss.Color("#7C3AED"))
	t.Focused.Description = t.Focused.Description.Foreground(lipgloss.Color("#9CA3AF"))
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(lipgloss.Color("#10B981"))
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(lipgloss.Color("#7C3AED"))

	return t
}

// QuickSetupForm is a simplified form for quick debate setup.
type QuickSetupForm struct {
	Topic string
}

// NewQuickSetupForm creates a minimal setup form.
func NewQuickSetupForm() *QuickSetupForm {
	return &QuickSetupForm{}
}

// Run displays the quick setup form.
func (f *QuickSetupForm) Run() error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("What should the council debate?").
				Placeholder("Enter your topic...").
				Value(&f.Topic).
				Validate(func(s string) error {
					if len(s) < 5 {
						return fmt.Errorf("topic must be at least 5 characters")
					}
					return nil
				}),
		),
	).WithTheme(councilTheme())

	return form.Run()
}

// MemberSelectForm allows selecting council members.
type MemberSelectForm struct {
	Members []string
	config  *config.Config
}

// NewMemberSelectForm creates a member selection form.
func NewMemberSelectForm(cfg *config.Config, currentMembers []string) *MemberSelectForm {
	return &MemberSelectForm{
		Members: currentMembers,
		config:  cfg,
	}
}

// Run displays the member selection form.
func (f *MemberSelectForm) Run() error {
	// Build member options
	var options []huh.Option[string]
	for id, modelCfg := range f.config.Models {
		displayName := fmt.Sprintf("%s (%s)", modelCfg.DisplayName, modelCfg.Provider)
		options = append(options, huh.NewOption(displayName, id))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Council Members").
				Description("Select which AIs will participate in the debate").
				Options(options...).
				Value(&f.Members).
				Validate(func(s []string) error {
					if len(s) < 2 {
						return fmt.Errorf("select at least 2 council members")
					}
					if len(s) > 6 {
						return fmt.Errorf("maximum 6 council members")
					}
					return nil
				}),
		),
	).WithTheme(councilTheme())

	return form.Run()
}

// truncate shortens a string to maxLen with ellipsis.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 4 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
