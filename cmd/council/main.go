// The Council of Legends - Council CLI
// Main entry point for the debate command
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/jxmullins/thekanbansociety/internal/config"
	"github.com/jxmullins/thekanbansociety/internal/debate"
	"github.com/jxmullins/thekanbansociety/internal/provider"
	"github.com/jxmullins/thekanbansociety/internal/tui"
)

var (
	cfgFile    string
	rounds     int
	mode       string
	members    []string
	verbose    bool
	noStream   bool
	useTUI     bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "council",
	Short: "The Council of Legends - Multi-AI Debate Platform",
	Long: `The Council of Legends brings together multiple AI models
to debate topics, synthesize perspectives, and reach consensus.

Run a debate with:
  council debate "Should we use microservices or monoliths?"

The council will engage in multiple rounds of discussion,
with each AI presenting arguments and responding to others.`,
}

var debateCmd = &cobra.Command{
	Use:   "debate [topic]",
	Short: "Start a council debate on a topic",
	Long: `Start a multi-AI debate on the specified topic.

The debate proceeds through:
1. Opening statements from each AI
2. Multiple rounds of rebuttals
3. Individual synthesis from each AI
4. Combined final verdict

Example:
  council debate "What's the best approach to error handling in Go?"
  council debate --rounds 5 --mode adversarial "Tabs vs spaces"`,
	Args: cobra.ExactArgs(1),
	RunE: runDebate,
}

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List available AI models",
	RunE:  runListModels,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Council of Legends v2.0.0 (Go)")
	},
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive debate setup wizard",
	Long: `Launch an interactive form to configure and start a debate.

This wizard guides you through:
- Entering a debate topic
- Selecting debate mode (collaborative, adversarial, socratic)
- Choosing the number of rounds
- Selecting which AI models participate`,
	RunE: runSetup,
}

var manageCmd = &cobra.Command{
	Use:   "manage",
	Short: "Interactive model management",
	Long: `Launch an interactive interface to manage AI models.

Features:
- View all configured models
- Test model connectivity
- Add new models
- Edit existing models`,
	RunE: runManage,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./config/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	debateCmd.Flags().IntVarP(&rounds, "rounds", "r", 0, "number of debate rounds (default from config)")
	debateCmd.Flags().StringVarP(&mode, "mode", "m", "collaborative", "debate mode: collaborative, adversarial, socratic")
	debateCmd.Flags().StringSliceVar(&members, "members", nil, "council members (default from config)")
	debateCmd.Flags().BoolVar(&noStream, "no-stream", false, "disable streaming output")
	debateCmd.Flags().BoolVar(&useTUI, "tui", false, "use interactive TUI mode")

	rootCmd.AddCommand(debateCmd)
	rootCmd.AddCommand(modelsCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(manageCmd)
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

func setupProviders(cfg *config.Config) *provider.Registry {
	registry := provider.NewRegistry()

	// Register all providers
	registry.Register(provider.NewAnthropicProvider(""))
	registry.Register(provider.NewOpenAIProvider(""))
	registry.Register(provider.NewGoogleProvider(""))
	registry.Register(provider.NewGroqProvider(""))
	registry.Register(provider.NewDeepSeekProvider(""))
	registry.Register(provider.NewMistralProvider(""))
	registry.Register(provider.NewXAIProvider(""))
	registry.Register(provider.NewOllamaProvider("", ""))
	registry.Register(provider.NewLMStudioProvider("", ""))

	// Register models from config
	registry.RegisterModels(cfg.Models)

	return registry
}

func runDebate(cmd *cobra.Command, args []string) error {
	topic := args[0]

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Override config with flags
	if rounds == 0 {
		rounds = cfg.Debate.DefaultRounds
	}

	councilMembers := members
	if len(councilMembers) == 0 {
		councilMembers = cfg.GetCouncilMembers()
	}

	registry := setupProviders(cfg)

	// Use TUI mode if requested
	if useTUI {
		tuiOpts := tui.Options{
			Topic:     topic,
			Mode:      mode,
			Rounds:    rounds,
			Members:   councilMembers,
			OutputDir: cfg.Output.DebatesDir,
		}

		tuiRunner := tui.NewRunner(registry, cfg)
		return tuiRunner.Run(tuiOpts)
	}

	// Create debate options for CLI mode
	opts := debate.Options{
		Topic:     topic,
		Mode:      debate.Mode(mode),
		Rounds:    rounds,
		Members:   councilMembers,
		Stream:    !noStream,
		Verbose:   verbose,
		OutputDir: cfg.Output.DebatesDir,
	}

	// Run the debate
	runner := debate.NewRunner(registry, cfg)
	return runner.Run(cmd.Context(), opts)
}

func runListModels(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	fmt.Println("Configured models:")
	fmt.Println()

	for id, model := range cfg.Models {
		fmt.Printf("  %-15s %s (%s)\n", id, model.DisplayName, model.Provider)
	}

	fmt.Println()
	fmt.Println("Default council:", cfg.GetCouncilMembers())

	return nil
}

func runSetup(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Show setup form
	form := tui.NewSetupForm(cfg)
	if err := form.Run(); err != nil {
		return err
	}

	// If user completed form, start the debate
	if form.Topic == "" {
		fmt.Println("Setup cancelled.")
		return nil
	}

	registry := setupProviders(cfg)

	// Run with TUI
	tuiOpts := form.GetOptions(cfg.Output.DebatesDir)
	tuiRunner := tui.NewRunner(registry, cfg)
	return tuiRunner.Run(tuiOpts)
}

func runManage(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	registry := setupProviders(cfg)
	return tui.RunModelManager(cfg, registry)
}
