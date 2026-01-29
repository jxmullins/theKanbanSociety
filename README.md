# The Kanban Society

A dynamic multi-AI collaboration platform with an interactive Kanban board TUI. Watch multiple AI agents work together in real-time, tracked on a visual board.

## Features

- **Interactive Kanban Board**: Track AI tasks across Backlog → In Progress → Review → Done
- **Multi-AI Collaboration**: Orchestrate Claude, Gemini, Codex, and more working together
- **Real-time Streaming**: Watch AI responses stream live into task cards
- **CLI Provider Support**: Use installed CLI tools (claude, gemini, codex) instead of API keys
- **Multiple Work Modes**: Consultation, pair programming, round-robin, divide & conquer
- **Debug Log Panel**: Live stream of commands and feedback (`~` to toggle)

## Installation

```bash
go build -o council ./cmd/council
go build -o team ./cmd/team
go build -o assess ./cmd/assess
```

## Usage

### Team Mode (Kanban TUI)

```bash
# Using CLI providers (recommended - no API keys needed)
./team "Build a REST API for user management" --tui --cli

# Using API providers
./team "Refactor the authentication module" --tui
```

### Council Mode (AI Debate)

```bash
./council debate "Should we use microservices or monolith?"
```

### Keyboard Controls (Team TUI)

| Key | Action |
|-----|--------|
| `h/l` or `←/→` | Move between columns |
| `j/k` or `↑/↓` | Move between cards |
| `Enter` | Open card details |
| `Tab` | Cycle active panel focus |
| `Space` | Pause/Resume |
| `d` | Mark user task done |
| `` ` `` or `~` | Toggle debug log |
| `?` | Help |
| `q` | Quit |

## CLI Providers

The Kanban Society can use your installed AI CLI tools:

- **Claude Code** (`claude`) - Anthropic's Claude
- **Gemini CLI** (`gemini`) - Google's Gemini
- **Codex CLI** (`codex`) - OpenAI's Codex

Use `--cli` flag to auto-detect and use available CLI tools.

## Architecture

```
thekanbansociety/
├── cmd/
│   ├── council/     # AI debate CLI
│   ├── team/        # Team collaboration with Kanban TUI
│   └── assess/      # Assessment CLI
├── internal/
│   ├── config/      # YAML configuration
│   ├── provider/    # AI provider adapters
│   ├── debate/      # Council debate orchestration
│   ├── team/        # Team collaboration logic
│   ├── tui/         # Bubble Tea TUI components
│   └── ...
└── config/          # Configuration files
```

## Configuration

Configuration is in YAML format at `config/config.yaml`.

## License

MIT License - see LICENSE file for details.
