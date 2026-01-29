// The Council of Legends - Team CLI
// Entry point for team collaboration mode
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/jxmullins/thekanbansociety/internal/config"
	"github.com/jxmullins/thekanbansociety/internal/provider"
	"github.com/jxmullins/thekanbansociety/internal/team"
	"github.com/jxmullins/thekanbansociety/internal/tui"
)

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
	useTUI          bool
	useCLI          bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "team [task]",
	Short: "The Council of Legends - Team Collaboration Mode",
	Long: `Team collaboration mode brings multiple AI models together to
complete tasks collaboratively.

The team is led by a Project Manager (PM) who:
- Analyzes the task
- Creates a work plan
- Selects the best work mode
- Coordinates team members
- Reviews and delivers the final output

Example:
  team "Build a REST API for user authentication"
  team "Design a database schema" --pm gpt --mode divide_conquer
  team "Review this codebase for security issues" --mode consultation`,
	Args: cobra.ExactArgs(1),
	RunE: runTeam,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./config/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.Flags().StringVar(&pm, "pm", "", "force a specific Project Manager (claude, gpt, gemini)")
	rootCmd.Flags().StringVar(&workMode, "mode", "", "work mode: pair_programming, consultation, round_robin, divide_conquer, free_form")
	rootCmd.Flags().StringSliceVar(&members, "members", nil, "team members (default: claude, gpt, gemini)")
	rootCmd.Flags().BoolVar(&includeArbiter, "with-arbiter", false, "include arbiter as 4th team member")
	rootCmd.Flags().StringVar(&checkpointLevel, "checkpoints", "all", "checkpoint level: all, major, none")
	rootCmd.Flags().BoolVar(&showCosts, "show-costs", false, "display estimated token costs")
	rootCmd.Flags().StringVarP(&outputDir, "output-dir", "o", "", "output directory for artifacts")
	rootCmd.Flags().BoolVar(&useTUI, "tui", false, "use interactive TUI with Kanban board")
	rootCmd.Flags().BoolVar(&useCLI, "cli", false, "use CLI tools (claude, gemini) instead of API keys")
}

func loadConfig() (*config.Config, error) {
	if cfgFile != "" {
		return config.Load(cfgFile)
	}

	// Try default locations
	locations := []string{
		"./config/config.yaml",
		"./config.yaml",
		filepath.Join(os.Getenv("HOME"), ".config/council/config.yaml"),
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return config.Load(loc)
		}
	}

	return nil, fmt.Errorf("no config file found, tried: %v", locations)
}

func setupProviders(cfg *config.Config, preferCLI bool) *provider.Registry {
	registry := provider.NewRegistry()

	if preferCLI {
		// Register CLI providers (use installed CLI tools instead of API keys)
		for _, p := range provider.DetectCLIProviders() {
			registry.Register(p)
		}
	} else {
		// Register API-based providers
		registry.Register(provider.NewAnthropicProvider(""))
		registry.Register(provider.NewOpenAIProvider(""))
		registry.Register(provider.NewGoogleProvider(""))
		registry.Register(provider.NewGroqProvider(""))
		registry.Register(provider.NewDeepSeekProvider(""))
		registry.Register(provider.NewMistralProvider(""))
		registry.Register(provider.NewXAIProvider(""))
		registry.Register(provider.NewOllamaProvider("", ""))
		registry.Register(provider.NewLMStudioProvider("", ""))
	}

	// Register models from config
	registry.RegisterModels(cfg.Models)

	return registry
}

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

	// If using CLI mode, override members with CLI provider names
	if useCLI && len(members) == 0 {
		teamMembers = []string{}
		for _, p := range provider.DetectCLIProviders() {
			teamMembers = append(teamMembers, p.Name())
		}
		if len(teamMembers) == 0 {
			return fmt.Errorf("no CLI tools found (install claude or gemini CLI)")
		}
	}

	registry := setupProviders(cfg, useCLI)

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

		return tui.RunTeamTUI(context.Background(), tuiOpts, cfg, registry)
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
