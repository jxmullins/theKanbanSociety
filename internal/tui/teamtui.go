// Package tui provides the Bubble Tea TUI for The Council of Legends.
package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jxmullins/thekanbansociety/internal/config"
	"github.com/jxmullins/thekanbansociety/internal/provider"
	"github.com/jxmullins/thekanbansociety/internal/team"
)

// TeamTUIOptions configures the team TUI.
type TeamTUIOptions struct {
	Task    string
	PM      string
	Mode    team.WorkMode
	Members []string
}

// RunTeamTUI runs the team collaboration with TUI.
func RunTeamTUI(ctx context.Context, opts TeamTUIOptions, cfg *config.Config, registry *provider.Registry) error {
	// Create team runner
	runner := team.NewRunner(registry, cfg)

	// Create Kanban model with events channel
	model := NewKanbanModel(opts.Task, runner.Events)

	// Create Bubble Tea program
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Run team in background
	teamOpts := team.Options{
		Task:            opts.Task,
		PM:              opts.PM,
		Mode:            opts.Mode,
		Members:         opts.Members,
		CheckpointLevel: team.CheckpointNone, // TUI handles checkpoints
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- runner.Run(ctx, teamOpts)
		close(runner.Events)
	}()

	// Run TUI
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// Check for team errors
	select {
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("team error: %w", err)
		}
	default:
	}

	return nil
}
