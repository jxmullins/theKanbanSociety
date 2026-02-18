# Copilot Instructions for The Kanban Society

## Project Overview

The Kanban Society is a dynamic multi-AI collaboration platform with an interactive Kanban board TUI built in Go. It orchestrates multiple AI agents working together in real-time, tracked on a visual board.

## Technology Stack

- **Language**: Go 1.25.6
- **TUI Framework**: Bubble Tea (charmbracelet/bubbletea)
- **CLI Framework**: Cobra (spf13/cobra)
- **Styling**: Lipgloss (charmbracelet/lipgloss)
- **Configuration**: YAML (gopkg.in/yaml.v3)

## Code Style & Conventions

### Go Conventions
- Follow standard Go formatting (`gofmt`)
- Use clear, descriptive package comments
- Export only what's necessary; keep implementation details private
- Prefer composition over inheritance
- Use interfaces for abstraction where appropriate

### Naming
- Use camelCase for unexported identifiers
- Use PascalCase for exported identifiers
- Keep acronyms uppercase (e.g., `AIID`, `PMSelector`)
- Use descriptive names that reflect purpose

### Package Organization
```
cmd/           # CLI entry points (council, team, assess)
internal/      # Private application code
  ├── config/  # YAML configuration handling
  ├── provider/# AI provider adapters
  ├── debate/  # Council debate orchestration
  ├── team/    # Team collaboration logic
  ├── tui/     # Bubble Tea TUI components
  └── ...
config/        # Configuration files
docs/          # Documentation
```

### Comments
- Add package-level comments for all packages
- Document exported functions, types, and constants
- Use `//` for single-line comments
- Keep comments concise and meaningful
- Focus on "why" rather than "what" when the code is clear

## AI Provider Integration

### Provider Pattern
- All AI providers implement a common interface
- Support both API-based and CLI-based providers
- CLI providers are preferred (no API keys needed)
- Supported providers: Claude, Gemini, Codex, GPT, O3, Groq, DeepSeek, Mistral

### Configuration
- Model registry in `config/config.yaml`
- Each model entry includes: provider, model name, display name
- Provider adapters in `internal/provider/`

## TUI Development

### Bubble Tea Patterns
- Models implement `tea.Model` interface
- Use message-based communication (commands and messages)
- Separate concerns: models, views, updates
- Handle window resize events
- Support keyboard navigation (h/j/k/l, arrows)

### Styling
- Use Lipgloss for consistent styling
- Define styles in centralized locations
- Support both light and dark themes where possible
- Use emoji for visual feedback (✨, 🎯, ✓, etc.)

### Kanban Board
- Four columns: Backlog → In Progress → Review → Done
- Cards represent AI tasks or work items
- Real-time streaming updates
- Support for pausing/resuming
- Debug log panel (toggle with `~`)

## Team Collaboration Modes

The system supports multiple work modes:
- **Consultation**: Sequential AI consultation
- **Pair Programming**: Two AIs working together
- **Round-Robin**: Multiple AIs taking turns
- **Divide & Conquer**: Parallel task distribution

## Testing & Validation

When adding new features:
- Build with: `go build -o council ./cmd/council` and similar
- Test manually with the TUI to ensure visual consistency
- Verify keyboard controls work as expected
- Test with `--cli` flag for CLI provider support
- Check debug log output for proper logging

## Common Tasks

### Adding a New AI Provider
1. Add model configuration to `config/config.yaml`
2. Implement provider adapter in `internal/provider/`
3. Register in provider registry
4. Add CLI detection if supporting CLI mode

### Adding TUI Components
1. Create model struct implementing `tea.Model`
2. Implement `Init()`, `Update()`, `View()` methods
3. Define keyboard bindings
4. Add styling with Lipgloss
5. Integrate with main TUI app

### Configuration Changes
- Update `config/config.yaml` for defaults
- Update config struct in `internal/config/`
- Add validation where appropriate
- Document in README.md

## Project-Specific Guidelines

1. **Real-time Updates**: Always consider streaming behavior when working with AI responses
2. **CLI Provider Support**: Prioritize CLI-based providers over API-based when possible
3. **Visual Feedback**: Use the Kanban board to show progress visually
4. **Error Handling**: Gracefully handle provider failures and continue execution
5. **Persona System**: Respect the persona/role system for AI agent behavior
6. **Project Management**: The PM (Project Manager) orchestrates team members
7. **Cost Tracking**: Support optional cost tracking for API-based providers

## Dependencies

Key dependencies to be aware of:
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/bubbles` - TUI components
- `github.com/charmbracelet/lipgloss` - Styling
- `github.com/spf13/cobra` - CLI framework
- `gopkg.in/yaml.v3` - YAML parsing

## Build & Run

```bash
# Build all commands
go build -o council ./cmd/council
go build -o team ./cmd/team
go build -o assess ./cmd/assess

# Run team mode with TUI
./team "Your task here" --tui --cli

# Run council debate
./council debate "Your debate topic"
```

## Additional Notes

- Keep dependencies minimal and well-justified
- Maintain compatibility with Go 1.25.6
- Follow the Bubble Tea patterns and best practices
- Ensure all changes work with both CLI and API providers
- Test interactive features in the TUI
- Preserve the visual consistency of the Kanban board
