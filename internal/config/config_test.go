package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
debate:
  default_rounds: 5
  turn_timeout: 120
models:
  test-model:
    provider: openai
    model: gpt-test
    display_name: Test Model
default_council:
  - test-model
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Debate.DefaultRounds != 5 {
		t.Errorf("DefaultRounds = %d, want 5", cfg.Debate.DefaultRounds)
	}

	if cfg.Debate.TurnTimeout != 120 {
		t.Errorf("TurnTimeout = %d, want 120", cfg.Debate.TurnTimeout)
	}

	model, ok := cfg.GetModel("test-model")
	if !ok {
		t.Error("GetModel() did not find test-model")
	}
	if model.Provider != "openai" {
		t.Errorf("Model.Provider = %s, want openai", model.Provider)
	}

	members := cfg.GetCouncilMembers()
	if len(members) != 1 || members[0] != "test-model" {
		t.Errorf("GetCouncilMembers() = %v, want [test-model]", members)
	}
}

func TestLoadDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Minimal config - should get defaults
	if err := os.WriteFile(configPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Debate.DefaultRounds != 3 {
		t.Errorf("DefaultRounds = %d, want default 3", cfg.Debate.DefaultRounds)
	}

	if cfg.Output.Format != "markdown" {
		t.Errorf("Output.Format = %s, want default markdown", cfg.Output.Format)
	}
}

func TestLoadPersona(t *testing.T) {
	tmpDir := t.TempDir()
	personaPath := filepath.Join(tmpDir, "test.yaml")

	personaContent := `
name: Test Persona
description: A test persona
system_prompt: You are a test.
traits:
  - analytical
  - creative
debate_style: collaborative
`
	if err := os.WriteFile(personaPath, []byte(personaContent), 0644); err != nil {
		t.Fatalf("failed to write test persona: %v", err)
	}

	persona, err := LoadPersona(personaPath)
	if err != nil {
		t.Fatalf("LoadPersona() error = %v", err)
	}

	if persona.Name != "Test Persona" {
		t.Errorf("Name = %s, want Test Persona", persona.Name)
	}

	if len(persona.Traits) != 2 {
		t.Errorf("Traits length = %d, want 2", len(persona.Traits))
	}
}

func TestLoadRole(t *testing.T) {
	tmpDir := t.TempDir()
	rolePath := filepath.Join(tmpDir, "test.yaml")

	roleContent := `
name: Test Role
description: A test role
system_prompt: You are a test role.
capabilities:
  - coding
  - reviewing
`
	if err := os.WriteFile(rolePath, []byte(roleContent), 0644); err != nil {
		t.Fatalf("failed to write test role: %v", err)
	}

	role, err := LoadRole(rolePath)
	if err != nil {
		t.Fatalf("LoadRole() error = %v", err)
	}

	if role.Name != "Test Role" {
		t.Errorf("Name = %s, want Test Role", role.Name)
	}

	if len(role.Capabilities) != 2 {
		t.Errorf("Capabilities length = %d, want 2", len(role.Capabilities))
	}
}
