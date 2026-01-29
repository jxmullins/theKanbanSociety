// Package config handles loading and managing configuration for The Council of Legends.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration.
type Config struct {
	Debate         DebateConfig            `yaml:"debate"`
	Execution      ExecutionConfig         `yaml:"execution"`
	Output         OutputConfig            `yaml:"output"`
	Context        ContextConfig           `yaml:"context"`
	Team           TeamConfig              `yaml:"team"`
	Models         map[string]ModelConfig  `yaml:"models"`
	DefaultCouncil []string                `yaml:"default_council"`
}

// DebateConfig holds debate-related settings.
type DebateConfig struct {
	DefaultRounds       int  `yaml:"default_rounds"`
	TurnTimeout         int  `yaml:"turn_timeout"`
	MaxResponseWords    int  `yaml:"max_response_words"`
	SummarizeAfterRound bool `yaml:"summarize_after_round"`
}

// ExecutionConfig holds execution-related settings.
type ExecutionConfig struct {
	ParallelOpening bool `yaml:"parallel_opening"`
	RetryOnFailure  bool `yaml:"retry_on_failure"`
	MaxRetries      int  `yaml:"max_retries"`
	RetryDelay      int  `yaml:"retry_delay"`
}

// OutputConfig holds output-related settings.
type OutputConfig struct {
	Format      string `yaml:"format"`
	SaveDebates bool   `yaml:"save_debates"`
	DebatesDir  string `yaml:"debates_dir"`
	UseColors   bool   `yaml:"use_colors"`
	Verbose     bool   `yaml:"verbose"`
}

// ContextConfig holds context management settings.
type ContextConfig struct {
	MaxContextChars    int  `yaml:"max_context_chars"`
	IncludeFullHistory bool `yaml:"include_full_history"`
}

// TeamConfig holds team mode settings.
type TeamConfig struct {
	ProjectsDir            string `yaml:"projects_dir"`
	DefaultCheckpointLevel string `yaml:"default_checkpoint_level"`
	DefaultShowCosts       bool   `yaml:"default_show_costs"`
}

// ModelConfig holds configuration for a single AI model.
type ModelConfig struct {
	Provider    string `yaml:"provider"`
	Model       string `yaml:"model"`
	DisplayName string `yaml:"display_name"`
	Endpoint    string `yaml:"endpoint,omitempty"`
	AuthEnvVar  string `yaml:"auth_env_var,omitempty"`
}

// Persona holds persona configuration.
type Persona struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	SystemPrompt string   `yaml:"system_prompt"`
	Traits       []string `yaml:"traits"`
	DebateStyle  string   `yaml:"debate_style"`
}

// Role holds team role configuration.
type Role struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	SystemPrompt string   `yaml:"system_prompt"`
	Capabilities []string `yaml:"capabilities"`
}

// Load reads configuration from the specified path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	// Set defaults
	cfg.setDefaults()

	return &cfg, nil
}

// LoadFromDir loads configuration from the standard config directory.
func LoadFromDir(configDir string) (*Config, error) {
	configPath := filepath.Join(configDir, "config.yaml")
	return Load(configPath)
}

// setDefaults applies default values for any unset fields.
func (c *Config) setDefaults() {
	if c.Debate.DefaultRounds == 0 {
		c.Debate.DefaultRounds = 3
	}
	if c.Debate.TurnTimeout == 0 {
		c.Debate.TurnTimeout = 180
	}
	if c.Debate.MaxResponseWords == 0 {
		c.Debate.MaxResponseWords = 400
	}
	if c.Execution.MaxRetries == 0 {
		c.Execution.MaxRetries = 2
	}
	if c.Execution.RetryDelay == 0 {
		c.Execution.RetryDelay = 5
	}
	if c.Output.Format == "" {
		c.Output.Format = "markdown"
	}
	if c.Output.DebatesDir == "" {
		c.Output.DebatesDir = "./debates"
	}
	if c.Context.MaxContextChars == 0 {
		c.Context.MaxContextChars = 8000
	}
	if c.Team.ProjectsDir == "" {
		c.Team.ProjectsDir = "./projects"
	}
	if c.Team.DefaultCheckpointLevel == "" {
		c.Team.DefaultCheckpointLevel = "all"
	}
}

// GetModel returns the model configuration for the given AI ID.
func (c *Config) GetModel(aiID string) (ModelConfig, bool) {
	model, ok := c.Models[aiID]
	return model, ok
}

// GetCouncilMembers returns the list of default council member IDs.
func (c *Config) GetCouncilMembers() []string {
	if len(c.DefaultCouncil) == 0 {
		return []string{"claude", "gpt", "gemini"}
	}
	return c.DefaultCouncil
}

// LoadPersona loads a persona from a YAML file.
func LoadPersona(path string) (*Persona, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading persona file: %w", err)
	}

	var persona Persona
	if err := yaml.Unmarshal(data, &persona); err != nil {
		return nil, fmt.Errorf("parsing persona file: %w", err)
	}

	return &persona, nil
}

// LoadPersonasFromDir loads all personas from a directory.
func LoadPersonasFromDir(dir string) (map[string]*Persona, error) {
	personas := make(map[string]*Persona)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return personas, nil
		}
		return nil, fmt.Errorf("reading personas directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		persona, err := LoadPersona(path)
		if err != nil {
			return nil, fmt.Errorf("loading persona %s: %w", entry.Name(), err)
		}

		// Use filename without extension as key
		key := entry.Name()[:len(entry.Name())-5]
		personas[key] = persona
	}

	return personas, nil
}

// LoadRole loads a role from a YAML file.
func LoadRole(path string) (*Role, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading role file: %w", err)
	}

	var role Role
	if err := yaml.Unmarshal(data, &role); err != nil {
		return nil, fmt.Errorf("parsing role file: %w", err)
	}

	return &role, nil
}

// LoadRolesFromDir loads all roles from a directory.
func LoadRolesFromDir(dir string) (map[string]*Role, error) {
	roles := make(map[string]*Role)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return roles, nil
		}
		return nil, fmt.Errorf("reading roles directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		role, err := LoadRole(path)
		if err != nil {
			return nil, fmt.Errorf("loading role %s: %w", entry.Name(), err)
		}

		// Use filename without extension as key
		key := entry.Name()[:len(entry.Name())-5]
		roles[key] = role
	}

	return roles, nil
}
